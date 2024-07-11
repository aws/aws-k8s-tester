import os
import time
import torch
import torch.distributed as dist
from torch.nn.parallel import DistributedDataParallel as DDP
from transformers import BertForPreTraining, BertTokenizer
from torch.utils.data import DataLoader, TensorDataset
import numpy as np


def create_dummy_data(tokenizer, num_samples=100, max_length=128):
    # Create dummy input data
    sentences = [
        "This is a dummy sentence number {}".format(i) for i in range(num_samples)
    ]
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
    input_ids, labels = mask_tokens(
        tokenized_inputs.input_ids, tokenizer, mlm_probability
    )

    # NSP task: create dummy pairs
    next_sentence_labels = torch.randint(0, 2, (num_samples,))

    return TensorDataset(
        input_ids, tokenized_inputs.attention_mask, labels, next_sentence_labels
    )


def mask_tokens(inputs, tokenizer, mlm_probability):
    labels = inputs.clone()
    probability_matrix = torch.full(labels.shape, mlm_probability)
    special_tokens_mask = [
        tokenizer.get_special_tokens_mask(val, already_has_special_tokens=True)
        for val in labels.tolist()
    ]
    probability_matrix.masked_fill_(
        torch.tensor(special_tokens_mask, dtype=torch.bool), value=0.0
    )
    masked_indices = torch.bernoulli(probability_matrix).bool()
    labels[~masked_indices] = -100  # We only compute loss on masked tokens

    inputs[masked_indices] = tokenizer.convert_tokens_to_ids(tokenizer.mask_token)

    return inputs, labels


def setup(rank, world_size):
    master_addr = os.environ["MASTER_ADDR"]
    master_port = os.environ["MASTER_PORT"]
    dist.init_process_group(
        "nccl",
        init_method=f"tcp://{master_addr}:{master_port}",
        rank=rank,
        world_size=world_size,
    )
    torch.cuda.set_device(rank)
    print(f"Process {rank} initialized, using GPU {rank}")


def cleanup():
    dist.destroy_process_group()


def train_bert(rank, world_size, model, tokenizer):
    setup(rank, world_size)

    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased").to(rank)
    ddp_model = DDP(model, device_ids=[rank])

    dataset = create_dummy_data(tokenizer)
    train_sampler = torch.utils.data.distributed.DistributedSampler(
        dataset, num_replicas=world_size, rank=rank
    )
    train_dataloader = DataLoader(dataset, sampler=train_sampler, batch_size=8)

    optimizer = torch.optim.AdamW(ddp_model.parameters(), lr=0.001)
    criterion = torch.nn.CrossEntropyLoss()

    start_time = time.time()

    for epoch in range(1):  # Short run for testing
        ddp_model.train()
        for batch in train_dataloader:
            optimizer.zero_grad()
            inputs, masks, labels, next_sentence_labels = batch
            inputs, masks, labels, next_sentence_labels = (
                inputs.to(rank),
                masks.to(rank),
                labels.to(rank),
                next_sentence_labels.to(rank),
            )
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


def main():
    # Pre-download model and tokenizer
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")

    rank = int(os.environ["OMPI_COMM_WORLD_RANK"])
    world_size = int(os.environ["OMPI_COMM_WORLD_SIZE"])
    train_bert(rank, world_size, model, tokenizer)


if __name__ == "__main__":
    main()
