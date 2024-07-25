import os
import time
import torch
from transformers import BertForPreTraining, BertTokenizer
from torch.utils.data import DataLoader, TensorDataset

# XLA imports
import torch_xla.core.xla_model as xm

# XLA imports for parallel loader and multi-processing
import torch_xla.distributed.parallel_loader as pl
from torch.utils.data.distributed import DistributedSampler

# Initialize XLA process group for torchrun
import torch_xla.distributed.xla_backend
torch.distributed.init_process_group('xla')


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




def train_bert(rank, world_size, model, tokenizer):
    device = xm.xla_device()
    model = model.to(device)
    dataset = create_dummy_data(tokenizer)

    # Prepare data loader
    train_dataloader = DataLoader(dataset, batch_size=8)
    # XLA MP: use MpDeviceLoader from torch_xla.distributed
    parallel_loader = pl.MpDeviceLoader(train_dataloader, device)

    optimizer = torch.optim.AdamW(model.parameters(), lr=0.001)
    criterion = torch.nn.CrossEntropyLoss()

    # Run the training loop
    print('----------Training ---------------')
    model.train()   
    # The neuron device needs to warm up before training to avoid low throughput numbers
    for epoch in range(5):  
        start_time = time.time()
        for batch in parallel_loader:
            optimizer.zero_grad()
            inputs, masks, labels, next_sentence_labels = batch
            outputs = model(
                input_ids=inputs,
                attention_mask=masks,
                labels=labels,
                next_sentence_label=next_sentence_labels,
            )
            loss = outputs.loss
            loss.backward()
            xm.optimizer_step(optimizer)
            xm.mark_step()
    end_time = time.time()
    training_time = end_time - start_time
    throughput = len(dataset) / training_time

    print(f"Process {rank} - Training time: {training_time:.2f} seconds")
    print(f"Process {rank} - Throughput: {throughput:.2f} samples/second")
    print('----------End Training ---------------')



def main():
    # XLA MP: get world size
    world_size = xm.xrt_world_size()
    rank=xm.get_ordinal()
    torch.manual_seed(0)

    print(f"Process started for rank {rank}. world_size: {world_size}")

    # Pre-download model and tokenizer
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased", force_download=True)

    print(f"successfully downloaded model and tokenizer for rank: {rank}")

    train_bert(rank, world_size, model, tokenizer)


if __name__ == "__main__":
    main()
