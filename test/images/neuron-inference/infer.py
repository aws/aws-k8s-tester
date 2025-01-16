import logging
import os
import sys
import time
import json
import subprocess
import random
import concurrent.futures
import numpy as np
from copy import deepcopy

import torch
import torch_neuronx
from torch.utils.data import DataLoader, TensorDataset
from transformers import BertForPreTraining, BertTokenizer

logging.basicConfig(
    level=logging.INFO,
    format='[%(asctime)s] [%(levelname)s] [%(name)s] %(message)s',
    handlers=[logging.StreamHandler(sys.stdout)]
)
logger = logging.getLogger("BERTNeuronInference")
logger.setLevel(logging.INFO) 

def get_neuron_monitor_stats():
    """
    Runs neuron-monitor command and returns the first JSON output as a dictionary.
    Also validates if the environment has Inferentia1/2 device and proper device count.
    
    Returns:
        dict: Parsed JSON output containing neuron monitor statistics
        
    Raises:
        RuntimeError: If neuron-monitor command is not found or fails to execute
        RuntimeError: If environment doesn't have proper Neuron support
        json.JSONDecodeError: If the output cannot be parsed as valid JSON
    """
    try:
        # Run neuron-monitor with timeout to get first output
        process = subprocess.Popen(
            ['neuron-monitor'], 
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        
        # Wait for first line of output
        output = process.stdout.readline()
        
        # Terminate the process since we only need first output
        process.terminate()
        process.wait()
        
        if not output:
            raise RuntimeError("No output received from neuron-monitor")
            
        # Parse JSON output
        stats = json.loads(output)
        
        # Check for Neuron hardware support
        hardware_info = stats.get('neuron_hardware_info', {})
        device_type = hardware_info.get('neuron_device_type', '').lower()
        neuroncore_per_device_count = hardware_info.get('neuroncore_per_device_count', 0)
        
        if neuroncore_per_device_count <= 0:
            raise RuntimeError(f"No Neuron devices found (neuroncore_per_device_count: {neuroncore_per_device_count})")
            
        return neuroncore_per_device_count
        
    except FileNotFoundError:
        raise RuntimeError("neuron-monitor command not found")
    except json.JSONDecodeError as e:
        raise RuntimeError(f"Failed to parse JSON output: {e}")
    except Exception as e:
        raise RuntimeError(f"Error running neuron-monitor: {e}")


def print_info(msg: str):
    """Helper function to prefix all info messages uniformly."""
    logger.info(f"[INFO] {msg}")


def print_warning(msg: str):
    """Helper function for warnings."""
    logger.warning(f"[WARNING] {msg}")


def print_error(msg: str):
    """Helper function for errors."""
    logger.error(f"[ERROR] {msg}")


def create_dummy_data(tokenizer, batch_size, num_samples=10000, max_length=128, seed=42):
    """
    Creates a realistic Next Sentence Prediction (NSP) dataset for BERT model testing.

    Args:
        tokenizer (BertTokenizer): instance used to tokenize the input sentences
        batch_size (int): specifying the size of each batch
        num_samples (int): specifying total number of samples to generate (default: 100)
        max_length (int): specifying maximum sequence length for tokenization (default: 128)
        seed (int): for random seed to ensure reproducibility (default: 42)

    Returns:
        TensorDataset containing:
            - input_ids (torcTensor):  of tokenized input sequences
            - attention_mask:  of attention masks
            - nsp_labels: Tensor of NSP labels (0 for random next sentence, 1 for actual next sentence)

    Notes:
        - Automatically adjusts num_samples to be a multiple of batch_size
        - Creates balanced dataset with 50% true next sentences and 50% random sentences
        - Uses a predefined set of sample sentences for generating pairs
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


def run_inference(model, tokenizer, batch_size, mode, n_models=2, n_threads=2):
    """
    Runs BERT model inference using Neuron runtime with dummy NSP (Next Sentence Prediction) data.

    Args:
        model (BertForPreTraining): model instance to be used for inference
        tokenizer (BertTokenizer): instance for processing input text
        batch_size (int): specifying batch size (8 for throughput mode, 1 for latency mode)
        mode (str): indicating inference mode ('throughput' or 'latency')
        n_models (int): number of models to spawn
        n_threads (int): number of threads for inference

    Returns:
        None, but prints performance metrics including:
        - Duration of the job
        - Average time per batch
        - Throughput (samples per second)
        - P50, P95, P99 latency 
        - Batch Size
        - Total Batches Processed
        - Total Inferences

    Notes:
        - Performance metrics are logged with prefix [BERT_INFERENCE_NEURON_METRICS]
        - Uses torch_neuronx for model compilation
        - Handles both throughput and latency testing modes
        - Runs inference with no gradient computation (torch.no_grad)    
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
    # to call the .trace() method and compile the Bert model to Neuron.
    _input_ids, _attention_masks, _output_ids = dataset.tensors
    _split_input_ids = torch.split(_input_ids, batch_size)[0]
    _split_attention_masks = torch.split(_attention_masks, batch_size)[0]
    batch_input = (_split_input_ids, _split_attention_masks)
    try:
        # When n_models are spanwed, the default behaviour by Neuron is to allocate one model
        # to one core. More details can be found here:
        # https://awsdocs-neuron.readthedocs-hosted.com/en/latest/frameworks/torch/torch-neuronx/programming-guide/inference/core-placement.html#default-core-allocation-placement
        _model_neuron = torch_neuronx.trace(model, batch_input)
        models_neuron = [deepcopy(_model_neuron) for _ in range(n_models-1)] + [_model_neuron]
        # model_neuron = torch_neuronx.trace(model, batch_input)
    except Exception as e:
        logger.exception(f"[ERROR] Failed to trace BERT model. Failed with error: {e}")
        raise e

    latencies = []
    rows_processed = 0

    # Thread task
    def task(model_neuron, batches):
        local_rows_processed = 0
        print_info(f"Total batches in this thread: {len(batches)}")
        for batch in batches:
            batch_input_tensor, batch_attention_tensor, _ = batch
            input_tuple = tuple([batch_input_tensor, batch_attention_tensor])
            start = time.time()
            with torch.no_grad():
                _ = model_neuron(*input_tuple)
            finish = time.time()
            latencies.append((finish - start) * 1000)
            local_rows_processed += len(batch_input_tensor)
        print_info(f"Total rows in this thread: {local_rows_processed}")
        return local_rows_processed

    all_batches = list(dataloader)
    batches_per_thread = len(all_batches) // n_threads
    thread_batches = [all_batches[i:i + batches_per_thread] for i in range(0, len(all_batches), batches_per_thread)]
    
    # If there are any remaining batches, add them to the last thread
    if len(thread_batches) > n_threads:
        thread_batches[-2].extend(thread_batches[-1])
        thread_batches.pop()

    # Submit tasks
    print_info(f"Starting Neuron inference with {n_threads} threads...")
    begin = time.time()
    with concurrent.futures.ThreadPoolExecutor(max_workers=n_threads) as pool:
        futures = []
        for i in range(n_threads):
            model_index = i % len(models_neuron)
            futures.append(pool.submit(task, models_neuron[model_index], thread_batches[i]))
        
        # Wait for all tasks to complete and sum up the processed rows
        for future in concurrent.futures.as_completed(futures):
            rows_processed += future.result()
    end = time.time()

    # Compute metrics
    boundaries = [50, 95, 99]
    percentiles = {}

    for boundary in boundaries:
        name = f'latency_p{boundary}'
        percentiles[name] = np.percentile(latencies, boundary)
    
    duration = end - begin
    inferences = rows_processed
    throughput = inferences / duration
    avg_time_per_batch = np.mean(latencies)

    # Print metrics
    print_info("Neuron inference completed.")

    # Print metrics to support old logging format
    print_info(
        "[BERT_INFERENCE_NEURON_METRICS] "
        f"mode={mode} "
        f"avg_time_per_batch={avg_time_per_batch:.6f} "
        f"throughput_samples_per_sec={throughput:.6f}"
    )

    # performance metrics
    print_info(f"[BERT_INFERENCE_NEURON_METRICS] mode={mode}")
    print_info(f"[BERT_INFERENCE_NEURON_METRICS] duration={duration:.6f}")
    print_info(f"[BERT_INFERENCE_NEURON_METRICS] avg_time_per_batch={avg_time_per_batch:.6f}")
    print_info(f"[BERT_INFERENCE_NEURON_METRICS] throughput_samples_per_sec={throughput:.6f}")

    # latency metrics
    for name, value in percentiles.items():
        print_info(f"[BERT_INFERENCE_NEURON_METRICS] {name}={value:.6f}")
    
    print_info(f"[BERT_INFERENCE_NEURON_METRICS] batch_size={batch_size}")
    print_info(f"[BERT_INFERENCE_NEURON_METRICS] total_batches_processed={len(latencies)}")
    print_info(f"[BERT_INFERENCE_NEURON_METRICS] total_inferences={inferences}")


def main():
    """Main entry"""
    print_info("Starting main()...")
    try:
        neuroncore_per_device_count = get_neuron_monitor_stats()
        print_info(f"Spawing a total of {neuroncore_per_device_count} models")
    except RuntimeError as e:
        print_error(f"Neuron environment not detected. Failed with error: {e}")
        sys.exit(1)

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
        model = BertForPreTraining.from_pretrained(model_name)

    except Exception as e:
        print_error(f"Failed to load model/tokenizer: {e}")
        sys.exit(1)
    print_info("Model and tokenizer loaded successfully.")

    run_inference(model, tokenizer, batch_size, mode, n_models=neuroncore_per_device_count)
    print_info("main() completed all steps successfully.")


if __name__ == "__main__":
    main()
