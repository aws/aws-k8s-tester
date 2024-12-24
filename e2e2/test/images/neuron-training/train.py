import os
import time
import torch
from transformers import BertForPreTraining, BertTokenizer
from torch.utils.data import DataLoader, TensorDataset

# Neuron-specific imports
import torch_neuron
import torch_neuron.distributed as dist_neuron

def create_dummy_data(tokenizer, num_samples=100, max_length=128):
    # Create dummy input data
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
    probability_matrix.masked_fill_(
        torch.tensor(special_tokens_mask, dtype=torch.bool), value=0.0
    )
    masked_indices = torch.bernoulli(probability_matrix).bool()
    labels[~masked_indices] = -100  # We only compute loss on masked tokens

    inputs[masked_indices] = tokenizer.convert_tokens_to_ids(tokenizer.mask_token)

    return inputs, labels

def setup(rank, world_size):
    # Initialize the Neuron distributed process group
    dist_neuron.init_process_group(backend='neuron', rank=rank, world_size=world_size)
    print(f"Process {rank} initialized for Neuron")

def cleanup():
    dist_neuron.destroy_process_group()

def train_bert(rank, world_size, model, tokenizer):
    setup(rank, world_size)

    device = torch_neuron.device(f"neuron:{rank}")
    model = model.to(device)

    ddp_model = torch.nn.parallel.DistributedDataParallel(model, device_ids=[rank])

    dataset = create_dummy_data(tokenizer)
    sampler = torch.utils.data.distributed.DistributedSampler(dataset, num_replicas=world_size, rank=rank)
    train_dataloader = DataLoader(dataset, batch_size=8, sampler=sampler)

    optimizer = torch.optim.AdamW(ddp_model.parameters(), lr=0.001)

    start_time = time.time()

    # Simple multi-epoch training loop
    for epoch in range(5):
        ddp_model.train()
        for batch in train_dataloader:
            optimizer.zero_grad()
            inputs, masks, labels, next_sentence_labels = batch
            inputs = inputs.to(device)
            masks = masks.to(device)
            labels = labels.to(device)
            next_sentence_labels = next_sentence_labels.to(device)

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

    # Return the throughput so rank 0 can print "Average Throughput" line.
    return throughput

def main():
    # Retrieve environment variables
    world_size = int(os.getenv("WORLD_SIZE", "1"))
    rank = int(os.getenv("RANK", "0"))
    num_neurons_per_node = int(os.getenv("NUM_NEURONS_PER_NODE", "8"))
    node_count = int(os.getenv("NODE_COUNT", "1"))

    print(f"Process started for rank {rank}. World Size: {world_size}")

    # Pre-download model and tokenizer
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")

    print(f"Successfully downloaded model and tokenizer for rank: {rank}")

    throughput = train_bert(rank, world_size, model, tokenizer)

    # Only rank 0 prints the "Average Throughput" line
    if rank == 0:
        print(f"Average Throughput: {throughput:.2f} samples/second")

if __name__ == "__main__":
    main()
