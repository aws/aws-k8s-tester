import logging
import os
import sys
import time
import random

import torch
import torch_neuronx
from torch.utils.data import DataLoader, TensorDataset
from transformers import BertForPreTraining, BertTokenizer

logging.basicConfig(
    level=logging.INFO,
    format='[%(asctime)s] [%(levelname)s] [%(name)s] %(message)s',
    handlers=[logging.StreamHandler(sys.stdout)]
)
logger = logging.getLogger("BERTInferenceNeuron")


def create_dummy_data(tokenizer, batch_size, num_samples=100, max_length=128, seed=42):
    """
    Creates a realistic NSP-style dataset (50% next-sentence, 50% random).
    Ensures num_samples is a multiple of batch_size.
    """
    random.seed(seed)

    if num_samples % batch_size != 0:
        adjusted = (num_samples // batch_size) * batch_size
        logger.info(
            f"[INFO] Adjusting num_samples from {num_samples} to {adjusted} "
            f"to ensure full batches."
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
        inputs.input_ids,        # shape: (batch_size, seq_len)
        inputs.attention_mask,   # shape: (batch_size, seq_len)
        torch.tensor(nsp_labels, dtype=torch.long)
    )


def run_inference(model, tokenizer, batch_size, mode):
    """
    1) Creates dummy NSP data
    2) Moves model and data to the XLA device ("xla") for Inf2 usage
    3) Defines a wrapper for torch_neuronx.trace(...) that expects 2 positional arguments
    4) Traces and then runs inference in a loop
    """
    logger.info("[INFO] About to create dummy data...")
    try:
        dataset = create_dummy_data(tokenizer, batch_size=batch_size)
    except Exception:
        logger.exception("[ERROR] Failed to create dummy data.")
        raise
    logger.info("[INFO] Dummy data creation completed.")

    dataloader = DataLoader(dataset, batch_size=batch_size)
    logger.info(f"[INFO] DataLoader created with {len(dataloader)} batches.")

    # The XLA device for Inf2 usage.
    device = torch.device("xla")
    logger.info(f"[INFO] Using device: {device}")

    logger.info("[INFO] Moving model to XLA device...")
    model.to(device)
    model.eval()
    logger.info("[INFO] Model moved to device and set to eval mode.")

    def bert_inference_func(input_ids, attention_mask):
        # BERT forward pass with two inputs
        return model(input_ids=input_ids, attention_mask=attention_mask)

    # Grab a sample batch to compile the model
    try:
        sample_inputs, sample_masks, _ = next(iter(dataloader))
    except StopIteration:
        logger.error("[ERROR] DataLoader returned no batches; cannot trace model.")
        raise

    logger.info("[INFO] Casting sample inputs to long and moving to device...")
    sample_inputs = sample_inputs.long().to(device)
    sample_masks = sample_masks.long().to(device)

    logger.info("[INFO] About to trace model with torch_neuronx.trace().")
    try:
        model_neuron = torch_neuronx.trace(
            bert_inference_func,
            (sample_inputs, sample_masks)
        )
    except Exception:
        logger.exception("[ERROR] Model tracing failed.")
        raise
    logger.info("[INFO] Model tracing completed successfully.")

    total_time = 0.0
    total_batches = len(dataloader)

    logger.info(f"[INFO] Starting Neuron inference loop with {total_batches} total batches...")
    with torch.no_grad():
        for batch_idx, batch in enumerate(dataloader):
            inputs, masks, _ = batch
            logger.info(f"[INFO] Processing batch {batch_idx}/{total_batches-1}.")

            inputs = inputs.long().to(device)
            masks = masks.long().to(device)

            start_time = time.time()
            try:
                _ = model_neuron(inputs, masks)
            except Exception:
                logger.exception(f"[ERROR] Inference failed on batch {batch_idx}.")
                raise
            batch_time = time.time() - start_time
            total_time += batch_time
            logger.info(f"[INFO] Batch {batch_idx} inference time: {batch_time:.4f} seconds.")

    if total_time == 0.0:
        logger.error("[ERROR] Inference produced 0 total_time, raising RuntimeError.")
        raise RuntimeError("No inference time recorded (total_time == 0).")

    avg_time_per_batch = total_time / total_batches
    throughput = (total_batches * batch_size) / total_time

    logger.info("[INFO] Neuron inference loop completed.")
    logger.info(
        "[BERT_INFERENCE_NEURON_METRICS] "
        f"mode={mode} "
        f"avg_time_per_batch={avg_time_per_batch:.6f} "
        f"throughput_samples_per_sec={throughput:.6f}"
    )


def main():
    """
    Main entry. Requires NEURON_RT_VISIBLE_CORES or fails.
    Loads a BERT model, moves it to XLA device, runs a traced inference.
    """
    logger.info("[INFO] Starting main()...")
    if "NEURON_RT_VISIBLE_CORES" not in os.environ:
        logger.error("[ERROR] Neuron environment not detected (NEURON_RT_VISIBLE_CORES not set). Exiting.")
        sys.exit(1)
    logger.info("[INFO] NEURON_RT_VISIBLE_CORES is set.")

    mode = os.environ.get("INFERENCE_MODE", "throughput").lower()
    if mode not in ["throughput", "latency"]:
        logger.warning(
            f"[WARNING] Unrecognized INFERENCE_MODE '{mode}'. "
            "Falling back to 'throughput'."
        )
        mode = "throughput"

    batch_size = 1 if mode == "latency" else 8
    logger.info(f"[INFO] Running Neuron inference in {mode} mode with batch size {batch_size}.")

    logger.info("[INFO] Loading tokenizer and model...")
    try:
        tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
        model = BertForPreTraining.from_pretrained("bert-base-uncased")
    except Exception:
        logger.exception("[ERROR] Failed to load model/tokenizer. Exiting.")
        sys.exit(1)
    logger.info("[INFO] Model and tokenizer loaded successfully.")

    run_inference(model, tokenizer, batch_size, mode)
    logger.info("[INFO] main() completed all steps successfully.")


if __name__ == "__main__":
    main()
