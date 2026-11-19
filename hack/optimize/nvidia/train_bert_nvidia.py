import os
import time
import torch
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

def train_bert(model, tokenizer, batch_sizes, device):
    model = model.to(device)
    model.train()

    dataset = create_dummy_data(tokenizer)
    for batch_size in batch_sizes:
        try:
            train_dataloader = DataLoader(dataset, batch_size=batch_size)
            optimizer = torch.optim.AdamW(model.parameters(), lr=0.001)
            for _ in range(2):
                for batch in train_dataloader:
                    optimizer.zero_grad()
                    inputs, masks, labels, next_sentence_labels = batch
                    inputs, masks, labels, next_sentence_labels = (
                        inputs.to(device),
                        masks.to(device),
                        labels.to(device),
                        next_sentenP0+r\P0+r\ce_labels.to(device),
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
                break
            print(f"Batch Size: {batch_size} Training complete.")
            break
        except RuntimeError as e:
            if 'out of memory' in str(e).lower():
                print(f"Batch Size {batch_size}: Out of Memory. Trying smaller batch size.")
                torch.cuda.empty_cache()
                continue
            else:
                raise e

def main():
    device = torch.device('cuda')
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")
    batch_sizes = [1024, 512, 256, 128, 64, 32, 16, 8]
    train_bert(model, tokenizer, batch_sizes, device)

if __name__ == "__main__":
    main()

