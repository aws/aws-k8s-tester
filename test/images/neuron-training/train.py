import os
import time
import random

import torch
import torch.distributed as dist

# === torch_xla imports for device and parallel loader ===
import torch_xla.core.xla_model as xm
import torch_xla.distributed.xla_backend
import torch_xla.distributed.parallel_loader as pl

from torch.utils.data import DataLoader, TensorDataset, DistributedSampler
from transformers import BertForPreTraining, BertTokenizer


# Initialize XLA process group for MPI-based environment
# (Expects OMPI / MPI environment variables or torchrun-style env.)
dist.init_process_group("xla")

def create_dummy_data(tokenizer, num_samples=100, max_length=128):
    """
    Creates dummy BERT pretraining data (MLM + NSP).
    """
    print(f"Creating dummy data: {num_samples} samples, max_length={max_length}")
    sentences = [f"This is a dummy sentence number {i}" for i in range(num_samples)]
    encodings = tokenizer(
        sentences,
        max_length=max_length,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )
    labels = encodings.input_ids.detach().clone()

    # Randomly mask some tokens for MLM
    mlm_probability = 0.15
    input_ids, labels = mask_tokens(encodings.input_ids, tokenizer, mlm_probability)

    # Dummy next-sentence prediction labels
    next_sentence_labels = torch.randint(0, 2, (num_samples,))

    return TensorDataset(input_ids, encodings.attention_mask, labels, next_sentence_labels)


def mask_tokens(inputs, tokenizer, mlm_probability):
    """
    Randomly mask tokens for MLM. Unmasked tokens => label = -100
    so we don't compute loss on them.
    """
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
    labels[~masked_indices] = -100
    inputs[masked_indices] = tokenizer.convert_tokens_to_ids(tokenizer.mask_token)

    return inputs, labels

def complete_epoch(rank, epoch, optimizer, parallel_loader, model):
    for step_idx, batch in enumerate(parallel_loader, start=1):
        optimizer.zero_grad()
        input_ids, attention_mask, mlm_labels, next_sentence_labels = batch

        outputs = model(
            input_ids=input_ids,
            attention_mask=attention_mask,
            labels=mlm_labels,
            next_sentence_label=next_sentence_labels,
        )
        loss = outputs.loss
        loss.backward()

        xm.optimizer_step(optimizer)
        xm.mark_step()

        if step_idx % 10 == 0:
            print(f"[Rank {rank}] - Epoch {epoch}, Step {step_idx}, Loss={loss.item():.4f}")

def main():
    # Retrieve rank/world_size from MPI environment or fallback to zero/one
    rank = int(os.environ.get("OMPI_COMM_WORLD_RANK", "0"))
    world_size = int(os.environ.get("OMPI_COMM_WORLD_SIZE", "1"))

    print(f"Starting train.py with rank={rank}, world_size={world_size}")

    # Seed everything for reproducibility
    SEED = 42
    random.seed(SEED)
    torch.manual_seed(SEED)

    # Acquire the XLA device for this rank
    device = xm.xla_device()
    print(f"[Rank {rank}] using device: {device}")

    # Preload model + tokenizer
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")
    print(f"[Rank {rank}]: Model & tokenizer loaded.")

    # Create dummy dataset
    dataset = create_dummy_data(tokenizer, num_samples=1000, max_length=128)

    # Shard dataset for each rank
    sampler = DistributedSampler(
        dataset,
        num_replicas=world_size,
        rank=rank,
        shuffle=True,
        drop_last=False,
    )
    train_loader = DataLoader(dataset, batch_size=32, sampler=sampler)

    # XLA parallel data loader
    parallel_loader = pl.MpDeviceLoader(train_loader, device)

    # Move model to XLA device
    model = model.to(device)

    optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3)

    # Let's do 5 epochs
    epochs = 5

    model.train()

    print(f"[Rank {rank}] - Starting warmup")
    warmup_start = time.time()
    complete_epoch(rank, 0, optimizer, parallel_loader, model)
    warump_time = time.time() - warmup_start
    print(f"[Rank {rank}] - Finished warmup in {warump_time:.2f}s")

    print(f"[Rank {rank}] - Starting training for {epochs} epochs...")

    start_time = time.time()
    epoch_times = []

    for epoch in range(1, epochs + 1):
        epoch_start_time = time.time()
        print(f"[Rank {rank}] - Epoch {epoch}/{epochs}")

        complete_epoch(rank, epoch, optimizer, parallel_loader, model)

        epoch_time = time.time() - epoch_start_time
        epoch_times.append(epoch_time)
        print(f"[Rank {rank}] - Epoch {epoch} done in {epoch_time:.2f}s")

    # Total training time
    total_time = time.time() - start_time
    print(f"[Rank {rank}] - All epochs complete in {total_time:.2f}s")

    # Each rank processes (dataset_size / world_size) * epochs samples
    local_samples = (len(dataset) / world_size) * epochs
    local_throughput = local_samples / total_time

    # Average epoch time (local)
    if epoch_times:
        avg_epoch_time = sum(epoch_times) / len(epoch_times)
    else:
        avg_epoch_time = 0.0

    print(
        f"[Rank {rank}] - local_samples={local_samples:.1f}, total_time={total_time:.2f}s, "
        f"local_throughput={local_throughput:.2f} samples/s, local_avg_epoch_time={avg_epoch_time:.2f}s"
    )

    print(f"[Rank {rank}] training complete. Exiting main().")


if __name__ == "__main__":
    main()
