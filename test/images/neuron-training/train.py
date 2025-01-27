import os
import time
import random

import torch
import torch.distributed as dist

# === torch_xla imports for device and parallel loader ===
import torch_xla.core.xla_model as xm
import torch_xla.runtime as xr
import torch_xla.distributed.xla_backend
import torch_xla.distributed.parallel_loader as pl

from torch.utils.data import DataLoader, TensorDataset, DistributedSampler
from transformers import BertForPreTraining, BertTokenizer

RANK = int(os.environ.get("RANK", 0))
WORLD_SIZE = int(os.environ.get("WORLD_SIZE", 1))

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

def complete_epoch(epoch, optimizer, parallel_loader, model):

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

        if step_idx % 10 == 0:
            print(f"[Rank {RANK}] - Epoch {epoch}, Step {step_idx}, Loss={loss.item():.4f}")

def main():
    dist.init_process_group(
        "xla",
        init_method="xla://"
    )

    # print info with xla runtime functions to sanity check run context correctly propagates to backend
    print(f"Starting train.py with rank={xr.global_ordinal()}, world_size={xr.world_size()}")

    # Seed everything for reproducibility
    SEED = 42
    random.seed(SEED)
    torch.manual_seed(SEED)

    device = xm.xla_device()

    # Preload model + tokenizer
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")
    print(f"[Rank {RANK}]: Model & tokenizer loaded.")

    # Create dummy dataset
    dataset = create_dummy_data(tokenizer, num_samples=1000, max_length=128)

    # Shard dataset for each RANK
    sampler = DistributedSampler(
        dataset,
        num_replicas=WORLD_SIZE,
        rank=RANK,
        shuffle=True,
        drop_last=False,
    )
    train_loader = DataLoader(dataset, batch_size=1024, sampler=sampler)

    # XLA parallel data loader
    parallel_loader = pl.MpDeviceLoader(train_loader, device)

    # Move model to XLA device
    model = model.to(device)

    optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3)

    # Let's do 5 epochs
    epochs = 5

    model.train()

    # TODO: precompile the model. This warmup is arbitrary based on observed behavior
    # neuronx-cc seems to recompile for the first 2 runs for some reason tbd
    print(f"[Rank {RANK}] - Starting warmup (2 repetitions of epoch 0)")
    warmup_start = time.time()
    complete_epoch(0, optimizer, parallel_loader, model)
    complete_epoch(0, optimizer, parallel_loader, model)
    warump_time = time.time() - warmup_start
    print(f"[Rank {RANK}] - Finished warmup in {warump_time:.2f}s")

    print(f"[Rank {RANK}] - Starting training for {epochs} epochs...")

    start_time = time.time()
    epoch_times = []

    for epoch in range(1, epochs + 1):
        epoch_start_time = time.time()
        print(f"[Rank {RANK}] - Epoch {epoch}/{epochs}")

        complete_epoch(epoch, optimizer, parallel_loader, model)

        epoch_time = time.time() - epoch_start_time
        epoch_times.append(epoch_time)

        print(f"[Rank {RANK}] - Epoch {epoch} done in {epoch_time:.2f}s")

    # Total training time
    total_time = time.time() - start_time
    print(f"[Rank {RANK}] - All epochs complete in {total_time:.2f}s")

    # Each rank processes (dataset_size / WORLD_SIZE) * epochs samples
    local_samples = (len(dataset) / WORLD_SIZE) * epochs
    local_throughput = local_samples / total_time

    # Average epoch time (local)
    if epoch_times:
        avg_epoch_time = sum(epoch_times) / len(epoch_times)
    else:
        avg_epoch_time = 0.0

    print(
        f"[Rank {RANK}] - local_samples={local_samples:.1f}, total_time={total_time:.2f}s, "
        f"local_throughput={local_throughput:.2f} samples/s, local_avg_epoch_time={avg_epoch_time:.2f}s"
    )

    print(f"[Rank {RANK}] training complete. Exiting main().")

if __name__ == "__main__":
    main()
