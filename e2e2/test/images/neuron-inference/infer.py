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


def create_dummy_data(
    tokenizer,
    batch_size,
    num_samples=100,
    max_length=128,
    seed=42
):
    """
    Creates a realistic NSP-style dataset:
      - 50% true next-sentence pairs
      - 50% random second sentences
    Ensures num_samples is a multiple of batch_size, for deterministic batching.
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
            idx_b = (idx_a + 1) % len(sample_sentences)
            nsp_labels.append(1)
        else:
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
    Runs a BERT inference workload on Neuron hardware:
      1) Creates/loads dummy NSP data
      2) Compiles (traces) the model with torch_neuronx
      3) Measures throughput and timing across batches
      4) Raises an error if total_time is 0
    """
    try:
        dataset = create_dummy_data(tokenizer, batch_size=batch_size)
    except Exception:
        logger.exception("[ERROR] Failed to create dummy data.")
        raise

    dataloader = DataLoader(dataset, batch_size=batch_size)

    try:
        sample_inputs, sample_masks, sample_labels = next(iter(dataloader))
        logger.info("[INFO] Tracing model with torch_neuronx.trace().")
        model_neuron = torch_neuronx.trace(
            model, (sample_inputs, sample_masks, sample_labels)
        )
    except StopIteration:
        logger.error("[ERROR] DataLoader returned no batches; cannot trace model.")
        raise
    except Exception:
        logger.exception("[ERROR] Model tracing failed.")
        raise

    total_time = 0.0
    total_batches = len(dataloader)

    logger.info("[INFO] Starting Neuron inference loop...")
    with torch.no_grad():
        for batch_idx, batch in enumerate(dataloader):
            inputs, masks, nsp_labels = batch
            start_time = time.time()
            try:
                _ = model_neuron(
                    input_ids=inputs,
                    attention_mask=masks,
                    next_sentence_label=nsp_labels
                )
            except Exception:
                logger.exception(f"[ERROR] Inference failed on batch {batch_idx}.")
                raise
            total_time += (time.time() - start_time)

    if total_time == 0.0:
        logger.error("[ERROR] Inference produced 0 total_time, raising RuntimeError.")
        raise RuntimeError("No inference time recorded (total_time == 0).")

    avg_time_per_batch = total_time / total_batches
    throughput = (total_batches * batch_size) / total_time

    logger.info(
        "[BERT_INFERENCE_NEURON_METRICS] "
        f"mode={mode} "
        f"avg_time_per_batch={avg_time_per_batch:.6f} "
        f"throughput_samples_per_sec={throughput:.6f}"
    )


def main():
    """
    Main entry for Neuron-based BERT inference. Checks for Neuron presence,
    determines mode, sets batch size, loads model, and calls run_inference().
    """
    if "NEURON_RT_VISIBLE_CORES" not in os.environ:
        logger.error("[ERROR] Neuron environment not detected (NEURON_RT_VISIBLE_CORES not set). Exiting.")
        sys.exit(1)

    mode = os.environ.get("INFERENCE_MODE", "throughput").lower()
    if mode not in ["throughput", "latency"]:
        logger.warning(
            f"[WARNING] Unrecognized INFERENCE_MODE '{mode}'. "
            "Falling back to 'throughput'."
        )
        mode = "throughput"

    batch_size = 1 if mode == "latency" else 8
    logger.info(f"[INFO] Running Neuron inference in {mode} mode with batch size {batch_size}.")

    try:
        tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
        model = BertForPreTraining.from_pretrained("bert-base-uncased")
        model.eval()
    except Exception:
        logger.exception("[ERROR] Failed to load model/tokenizer. Exiting.")
        sys.exit(1)

    run_inference(model, tokenizer, batch_size, mode)


if __name__ == "__main__":
    main()
