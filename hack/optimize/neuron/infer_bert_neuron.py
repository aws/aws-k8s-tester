import os

# Unset XLA_FLAGS to avoid GPU-specific issues on Neuron
os.environ.pop('XLA_FLAGS', None)

import torch
import torch_neuronx
from transformers import BertTokenizer, BertForPreTraining
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
        tokenized_inputP1+rOQ\P1+rOR\P1+rOS\s.input_ids,
        tokenized_inputs.attention_mask,
        labels,
        next_sentence_labels,
    )

def infer_bert_neuron(model, tokenizer, batch_sizes, device):
    dataset = create_dummy_data(tokenizer)
    results = []

    for batch_size in batch_sizes:
        try:
            dataloader = DataLoader(dataset, batch_size=batch_size)
            start_time = time.time()
            for batch in dataloader:
                inputs, masks, labels, next_sentence_labels = batch
                inputs, masks = inputs.to(device), masks.to(device)
                outputs = model(input_ids=inputs, attention_mask=masks)
            end_time = time.time()
            inference_time = end_time - start_time
            throughput = len(dataset) / inference_time

            print(f"Batch Size: {batch_size}")
            print(f"Inference time: {inference_time:.2f} seconds")
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
    device = torch.device("xla")
    tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
    model = BertForPreTraining.from_pretrained("bert-base-uncased")

    example_inputs = torch.randint(0, 2000, (1, 128)).to(device)
    model_neuron = torch_neuronx.trace(model, example_inputs)

    batch_sizes = [128, 64, 32, 16, 8]
    infer_bert_neuron(model_neuron, tokenizer, batch_sizes, device)

if __name__ == "__main__":
    main()

