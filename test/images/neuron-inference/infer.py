import os
import sys
import time
import random

import torch
import torch_neuronx
from torch.utils.data import DataLoader, TensorDataset
from transformers import BertForPreTraining, BertTokenizer


def print_info(msg: str):
    """Helper function to prefix all info messages uniformly."""
    print(f"[INFO] {msg}")


def print_warning(msg: str):
    """Helper function for warnings."""
    print(f"[WARNING] {msg}")


def print_error(msg: str):
    """Helper function for errors."""
    print(f"[ERROR] {msg}")


def create_dummy_data(tokenizer, batch_size, num_samples=100, max_length=128, seed=42):
    """
    Creates a realistic NSP-style dataset (50% next-sentence, 50% random).
    Ensures num_samples is a multiple of batch_size.
    """
    random.seed(seed)

    if num_samples % batch_size != 0:
        adjusted = (num_samples // batch_size) * batch_size
        print_info(
            f"Adjusting num_samples from {num_samples} to {adjusted} "
            "to ensure full batches."
        )
        num_samples = adjusted

    sample_sentences = [
        "The dog loves playing fetch in the park.",
        "Artificial intelligence is reshaping the future.",
        "Movies with complex storylines can be very engaging.",
        "This restaurant serves an amazing brunch on weekends.",
        "Many researchers are exploring neural network architectures.",
        "A day at the beach can reduce stress and improve well-being.",
        "ChatGPT is a popular large language model by OpenAI.",
        "The annual developer conference showcased innovative technologies.",
        "Hiking in the mountains offers both challenge and relaxation.",
        "Robotics and automation are revolutionizing many industries.",
    ]

    sentences_a = []
    sentences_b = []
    nsp_labels = []

    for _ in range(num_samples):
        idx_a = random.randint(0, len(sample_sentences) - 1)
        if random.random() < 0.5:
            # “True” next sentence
            idx_b = (idx_a + 1) % len(sample_sentences)
            nsp_labels.append(1)
        else:
            # Random sentence
            idx_b = random.randint(0, len(sample_sentences) - 1)
            nsp_labels.append(0)

        sentences_a.append(sample_sentences[idx_a])
        sentences_b.append(sample_sentences[idx_b])

    inputs = tokenizer(
        sentences_a,
        sentences_b,
        max_length=max_length,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )

    return TensorDataset(
        inputs.input_ids,
        inputs.attention_mask,
        torch.tensor(nsp_labels, dtype=torch.long)
    )


def run_inference(model, tokenizer, batch_size, mode):
    """
    1) Creates dummy NSP data
    2) Moves model and data to the XLA device ("xla") for Inf2 usage
    3) Defines a wrapper for torch_neuronx.trace(...) that expects 2 positional arguments
    4) Traces and then runs inference in a loop
    """
    print_info("About to create dummy data...")
    try:
        dataset = create_dummy_data(tokenizer, batch_size=batch_size)
    except Exception as e:
        print_error(f"Failed to create dummy data: {e}")
        raise

    print_info("Dummy data creation completed.")

    dataloader = DataLoader(dataset, batch_size=batch_size)
    
    # First compile the model for Neuron: 
    # Since we run inference in batches, we must first
    # split the dataset into the size of input expected in a
    # single batch. This input signature would then be used
    # to call the .trace() method and compile the Bert model to Neuron
    _input_ids, _attention_masks, _output_ids = dataset.tensors
    _split_input_ids = torch.split(_input_ids, batch_size)[0]
    _split_attention_masks = torch.split(_attention_masks, batch_size)[0]

    batch_input = (_split_input_ids, _split_attention_masks)
    model_neuron = torch_neuronx.trace(model, batch_input)

    print_info(f"DataLoader created with {len(dataloader)} batches.")


    """
    # The XLA device for Inf2 usage.
    device = torch.device("xla")
    print_info(f"Using device: {device}")
    
    print_info("Moving model to XLA device...")
    model.to(device)
    model.eval()
    print_info("Model moved to device and set to eval mode.")
    """

    """
    def bert_inference_func(input_ids, attention_mask):
        # BERT forward pass with two inputs
        return model(input_ids=input_ids, attention_mask=attention_mask)

    # Grab a sample batch to compile the model
    try:
        sample_inputs, sample_masks, _ = next(iter(dataloader))
    except StopIteration:
        print_error("DataLoader returned no batches; cannot trace model.")
        raise RuntimeError("No data to perform tracing.")

    print_info("Casting sample inputs to long and moving to device...")
    sample_inputs = sample_inputs.long().to(device)
    sample_masks = sample_masks.long().to(device)

    print_info("About to trace model with torch_neuronx.trace()...")
    try:
        model_neuron = torch_neuronx.trace(
            bert_inference_func,
            (sample_inputs, sample_masks)
        )
    except Exception as e:
        print_error(f"Model tracing failed: {e}")
        raise
    """
    print_info("Model tracing completed successfully.")

    total_time = 0.0
    total_batches = len(dataloader)

    print_info(f"Starting Neuron inference loop with {total_batches} total batches...")
    with torch.no_grad():
        for batch_idx, batch in enumerate(dataloader):
            input_tuple = tuple(batch[:2])
            print_info(f"Processing batch {batch_idx}/{total_batches - 1}.")
            start_time = time.time()
            try:
                _ = model_neuron(*input_tuple)
            except Exception as e:
                print_error(f"Inference failed on batch {batch_idx}: {e}")
                raise
            batch_time = time.time() - start_time
            total_time += batch_time
            print_info(f"Batch {batch_idx} inference time: {batch_time:.4f} seconds.")

    if total_time == 0.0:
        print_error("Inference produced 0 total_time. No inference time recorded.")
        raise RuntimeError("No inference time recorded (total_time == 0).")

    avg_time_per_batch = total_time / total_batches
    throughput = (total_batches * batch_size) / total_time

    print_info("Neuron inference loop completed.")
    print_info(
        f"[BERT_INFERENCE_NEURON_METRICS] mode={mode} "
        f"avg_time_per_batch={avg_time_per_batch:.6f} "
        f"throughput_samples_per_sec={throughput:.6f}"
    )


def main():
    """Main entry. Requires NEURON_RT_VISIBLE_CORES or fails."""
    print_info("Starting main()...")
    """
    if "NEURON_RT_VISIBLE_CORES" not in os.environ:
        print_error("Neuron environment not detected (NEURON_RT_VISIBLE_CORES not set). Exiting.")
        sys.exit(1)
    print_info("NEURON_RT_VISIBLE_CORES is set.")
    """
    mode = os.environ.get("INFERENCE_MODE", "throughput").lower()
    if mode not in ["throughput", "latency"]:
        print_warning(
            f"Unrecognized INFERENCE_MODE '{mode}'. "
            "Falling back to 'throughput'."
        )
        mode = "throughput"

    batch_size = 1 if mode == "latency" else 8
    print_info(f"Running Neuron inference in {mode} mode with batch size {batch_size}.")

    print_info("Loading tokenizer and model...")
    try:
        model_name = "bert-base-uncased"
        tokenizer = BertTokenizer.from_pretrained(model_name)
        model = BertForPreTraining.from_pretrained(model_name, torchscript=True)
    except Exception as e:
        print_error(f"Failed to load model/tokenizer: {e}")
        sys.exit(1)
    print_info("Model and tokenizer loaded successfully.")

    run_inference(model, tokenizer, batch_size, mode)
    print_info("main() completed all steps successfully.")


if __name__ == "__main__":
    main()
