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
    return TensorDataset(
        tokenized_inputs.input_ids,
        tokenized_inputs.attention_mask
    )

def inference_bert(model, tokenizer, batch_sizes, device):
    model = model.to(device)
    model.eval()

    dataset = create_dummy_data(tokenizer)
    for batch_size in batch_sizes:
        try:
            inference_dataloader = DataLoader(dataset, batch_size=batch_size)
            start_time = time.time()
            with torch.no_grad():
                for batch in inference_dataloader:
                    inputs, masks = batch
                    inputs, masks = inputs.to(device), masks.to(device)
                    outputs = model(input_ids=inputs, attention_mask=masks)
            end_time = time.time()
            print(f"Batch Size: {batch_size} Inference time: {end_time - start_time:.2f} seconds")
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
    inference_bert(model, tokenizer, batch_sizes, device)

if __name__ == "__main__":
    main()

