import time

import torch
import torch_neuronx
from transformers import BertForPreTraining, BertTokenizer
from torch.utils.data import DataLoader, TensorDataset

import torch_xla.core.xla_model as xm

def create_dummy_data(tokenizer, num_samples=100, max_length=128):
    sentences = [f"This is a dummy sentence number {i}" for i in range(num_samples)]
    tok = tokenizer(
        sentences,
        max_length=max_length,
        padding="max_length",
        truncation=True,
        return_tensors="pt"
    )
    labels = tok.input_ids.clone()

    # Masked LM
    mlm_probability = 0.15
    input_ids, labels = mask_tokens(tok.input_ids, tokenizer, mlm_probability)

    # Next Sentence Prediction
    next_sentence_labels = torch.randint(0, 2, (num_samples,))

    return TensorDataset(input_ids, tok.attention_mask, labels, next_sentence_labels)

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

def init_distributed():
    rank = xm.get_ordinal()
    world_size = xm.xrt_world_size()
    return rank, world_size

def main():
    rank, world_size = init_distributed()
    print(f"[Rank {rank}] init_process_group done. world_size={world_size}")

    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model_cpu = BertForPreTraining.from_pretrained("bert-base-uncased")
    print(f"[Rank {rank}] Model loaded.")

    example_inputs = (
        torch.zeros([8, 128], dtype=torch.int64),
        torch.zeros([8, 128], dtype=torch.int64),
    )
    # Trace on CPU, then move to XLA device:
    model_neuron = torch_neuronx.trace(model_cpu, example_inputs)
    print(f"[Rank {rank}] Model compiled to Neuron IR.")

    # Move traced model onto the XLA device
    device = xm.xla_device()
    model_neuron = model_neuron.to(device)

    ddp_model = model_neuron

    dataset = create_dummy_data(tokenizer)
    sampler = torch.utils.data.distributed.DistributedSampler(dataset, num_replicas=world_size, rank=rank)
    train_loader = DataLoader(dataset, batch_size=8, sampler=sampler)

    optimizer = torch.optim.AdamW(ddp_model.parameters(), lr=0.001)

    start_time = time.time()
    for epoch in range(2):
        ddp_model.train()
        for batch in train_loader:
            optimizer.zero_grad()
            inputs, masks, labels, next_sentence_labels = batch
            # Move batch to XLA device too
            inputs = inputs.to(device)
            masks = masks.to(device)
            labels = labels.to(device)
            next_sentence_labels = next_sentence_labels.to(device)

            outputs = ddp_model(
                input_ids=inputs,
                attention_mask=masks,
                labels=labels,
                next_sentence_label=next_sentence_labels
            )
            loss = outputs.loss
            loss.backward()
            # Use xm.optimizer_step() for XLA
            xm.optimizer_step(optimizer)
            # Mark step to sync
            xm.mark_step()
    end_time = time.time()

    throughput = len(dataset) / (end_time - start_time)
    print(f"[Rank {rank}] Training time: {end_time - start_time:.2f}s. Throughput={throughput:.2f} samples/s")

    if rank == 0:
        print(f"Average Throughput: {throughput:.2f} samples/second")


if __name__ == "__main__":
    main()
