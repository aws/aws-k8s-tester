import os
import time
import torch
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
        input_ids, tokenized_inputs.attention_mask, next_sentence_labels
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


def run_inference(model, tokenizer, batch_size, mode):
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    model.to(device)
    model.eval()

    dataset = create_dummy_data(tokenizer)
    dataloader = DataLoader(dataset, batch_size=batch_size)

    total_time = 0
    total_batches = len(dataloader)

    with torch.no_grad():
        for batch in dataloader:
            inputs, masks, next_sentence_labels = batch
            inputs, masks, next_sentence_labels = (
                inputs.to(device),
                masks.to(device),
                next_sentence_labels.to(device),
            )

            start_time = time.time()
            outputs = model(
                input_ids=inputs,
                attention_mask=masks,
                next_sentence_label=next_sentence_labels,
            )
            end_time = time.time()

            total_time += end_time - start_time

    avg_time_per_batch = total_time / total_batches
    throughput = (total_batches * batch_size) / total_time
    
    print(f"Inference Mode: {mode}")
    print(f"Average time per batch: {avg_time_per_batch:.4f} seconds")
    print(f"Throughput: {throughput:.2f} samples/second")


def main():
    # Verify GPU availability
    if not torch.cuda.is_available():
        raise RuntimeError("GPU isnot available. Exiting")

    print("GPU is available")

    # Pre-download model and tokenizer
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")

    mode = os.environ.get("INFERENCE_MODE", "throughput").lower()
    batch_size = 1 if mode == "latency" else 8

    print(f"Running inference in {mode} mode with batch size {batch_size}")
    run_inference(model, tokenizer, batch_size, mode)


if __name__ == "__main__":
    main()
