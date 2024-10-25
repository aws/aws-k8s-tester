import os

# Unset XLA_FLAGS to avoid GPU-specific issues on Neuron
os.environ.pop('XLA_FLAGS', None)

import time
import torch
import torch_xla
import torch_xla.core.xla_model as xm
from transformers import BertForPreTraining, BertTokenizer
from torch.utils.data import DataLoader, TensorDataset

def create_dummy_data(tokenizer, num_samples=1000, max_length=128):
    sentences = [
        f"This is a dummy sentence number {i}" for i in range(num_samples)
    ]
    tokenized_inputs = tokenizer(
        sentences,
        max_length=max_length,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )
    labels = tokenized_inputs.input_ids.detach().clone()
    next_sentence_labels = torch.randint(0, 2, (num_samples,))
    return TensorDataset(
        tokenized_inputs.input_ids,
        tokenized_inputs.attention_mask,
        labels,
        next_sentence_labels,
    )

def train_bert_neuron(model, tokenizer, batch_sizes, device):
    model.train()
    model.to(device)

    dataset = create_dummy_data(tokenizer)
    results = []

    for batch_size in batch_sizes:
        try:
            train_dataloader = DataLoader(dataset, batch_size=batch_size, shuffle=True)
            optimizer = torch.optim.AdamW(model.parameters(), lr=0.001)
            
            # Measure training time for throughput calculation
            start_time = time.time()
            for batch in train_dataloader:
                optimizer.zero_grad()
                inputs, masks, labels, next_sentence_labels = batch
                inputs, masks, labels, next_sentence_labels = (
                    inputs.to(device),
                    masks.to(device),
                    labels.to(device),
                    next_sentence_labels.to(device),
                )
                outputs = model(
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

            print(f"Batch Size: {batch_size}")
            print(f"Training time: {training_time:.2f} seconds")
            print(f"Throughput: {throughput:.2f} samples/second")

            results.append({
                'batch_size': batch_size,
                'throughput': throughput,
            })
            break  # Exit after successful batch size

        except RuntimeError as e:
            if 'out of memory' in str(e).lower():
                print(f"Batch Size {batch_size}: Out of Memory. Trying smaller batch size.")
                torch.cuda.empty_cache()
                continue
            else:
                raise e

    print("Optimal Batch Size Found:")
    for res in results:
        print(f"Batch Size: {res['batch_size']}, Throughput: {res['throughput']:.2f} samples/sec")

def main():
    device = xm.xla_device()

    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")

    batch_sizes = [128, 64, 32, 16, 8]

    train_bert_neuron(model, tokenizer, batch_sizes, device)

if __name__ == "__main__":
    main()

