import os
import time
import torch
import torch.distributed as dist
from torch.nn.parallel import DistributedDataParallel as DDP
from transformers import BertForPreTraining, BertTokenizer
from torch.utils.data import DataLoader, TensorDataset
import numpy as np


def create_dummy_data(tokenizer, num_samples=100, max_length=128):
    sentences = [f"This is a dummy sentence number {i}" for i in range(num_samples)]
    tokenized_inputs = tokenizer(
        sentences,
        max_length=max_length,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )
    labels = tokenized_inputs.input_ids.detach().clone()

    # MLM task: randomly mask some tokens
    mlm_probability = 0.15
    input_ids, labels = mask_tokens(tokenized_inputs.input_ids, tokenizer, mlm_probability)

    # NSP task: create dummy pairs
    next_sentence_labels = torch.randint(0, 2, (num_samples,))

    return TensorDataset(input_ids, tokenized_inputs.attention_mask, labels, next_sentence_labels)


def mask_tokens(inputs, tokenizer, mlm_probability):
    labels = inputs.clone()
    probability_matrix = torch.full(labels.shape, mlm_probability)
    special_tokens_mask = [
        tokenizer.get_special_tokens_mask(val, already_has_special_tokens=True)
        for val in labels.tolist()
    ]
    probability_matrix.masked_fill_(torch.tensor(special_tokens_mask, dtype=torch.bool), value=0.0)
    masked_indices = torch.bernoulli(probability_matrix).bool()
    labels[~masked_indices] = -100
    inputs[masked_indices] = tokenizer.convert_tokens_to_ids(tokenizer.mask_token)
    return inputs, labels


def setup(rank, world_size, local_rank):
    master_addr = os.environ["MASTER_ADDR"]
    master_port = os.environ["MASTER_PORT"]
    dist.init_process_group(
        "nccl",
        init_method=f"tcp://{master_addr}:{master_port}",
        rank=rank,
        world_size=world_size,
    )
    torch.cuda.set_device(local_rank)
    print(f"Process {rank} initialized, using GPU {local_rank}")


def cleanup():
    dist.destroy_process_group()


def train_bert(rank, world_size, local_rank, model, tokenizer):
    setup(rank, world_size, local_rank)

    model = model.to(local_rank)
    ddp_model = DDP(model, device_ids=[local_rank])

    dataset = create_dummy_data(tokenizer)
    train_dataloader = DataLoader(dataset, batch_size=8)

    optimizer = torch.optim.AdamW(ddp_model.parameters(), lr=0.001)

    start_time = time.time()

    # Simple single-epoch training loop
    for epoch in range(1):
        ddp_model.train()
        for batch in train_dataloader:
            optimizer.zero_grad()
            inputs, masks, labels, next_sentence_labels = batch
            inputs = inputs.to(local_rank)
            masks = masks.to(local_rank)
            labels = labels.to(local_rank)
            next_sentence_labels = next_sentence_labels.to(local_rank)

            outputs = ddp_model(
                input_ids=inputs,
                attention_mask=masks,
                labels=labels,
                next_sentence_label=next_sentence_labels,
            )
            loss = outputs.loss
            loss.backward()
            optimizer.step()

    end_time = time.time()
    training_time = end_time - start_time
    throughput = len(dataset) / training_time

    print(f"Process {rank} - Training time: {training_time:.2f} seconds")
    print(f"Process {rank} - Throughput: {throughput:.2f} samples/second")

    cleanup()

    return throughput


def main():
    # Retrieve environment variables
    rank = int(os.getenv("OMPI_COMM_WORLD_RANK", "0"))
    world_size = int(os.getenv("OMPI_COMM_WORLD_SIZE", "1"))
    num_gpus_per_node = int(os.getenv("NUM_GPUS_PER_NODE", "8"))
    local_rank = rank % num_gpus_per_node

    print(f"Process started for rank {rank} with local rank {local_rank}")

    # Pre-download model and tokenizer
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")

    print(f"successfully downloaded model and tokenizer for rank: {rank}")

    throughput = train_bert(rank, world_size, local_rank, model, tokenizer)

    # Only rank 0 prints the "Average Throughput" line
    if rank == 0:
        print(f"Average Throughput: {throughput:.2f} samples/second")


if __name__ == "__main__":
    main()
