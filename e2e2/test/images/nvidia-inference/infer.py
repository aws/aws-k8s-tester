import logging
import os
import sys
import time
import random

import torch
from torch.utils.data import DataLoader, TensorDataset
from transformers import BertForPreTraining, BertTokenizer

logging.basicConfig(
    level=logging.INFO,
    format='[%(asctime)s] [%(levelname)s] [%(name)s] %(message)s',
    handlers=[logging.StreamHandler(sys.stdout)]
)
logger = logging.getLogger("BERTInference")


def create_dummy_data(tokenizer, batch_size, num_samples=100, max_length=128):
    """
    Creates a realistic NSP-style dataset:
      - 50% true next-sentence pairs
      - 50% random second sentences
    Ensures the final number of samples is a multiple of 'batch_size'.
    """
    # Align total samples to a multiple of batch_size
    if num_samples % batch_size != 0:
        adjusted = (num_samples // batch_size) * batch_size
        logger.info(
            f"[INFO] Adjusting num_samples from {num_samples} to {adjusted} "
            f"to ensure full batches."
        )
        num_samples = adjusted

    # Some sentences to simulate more realistic text
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
    nsp_labels = []  # 1 = next, 0 = random

    for _ in range(num_samples):
        idx_a = random.randint(0, len(sample_sentences) - 1)

        # 50% chance for the second sentence to be the 'true' next
        if random.random() < 0.5:
            idx_b = (idx_a + 1) % len(sample_sentences)
            nsp_labels.append(1)
        else:
            idx_b = random.randint(0, len(sample_sentences) - 1)
            nsp_labels.append(0)

        sentences_a.append(sample_sentences[idx_a])
        sentences_b.append(sample_sentences[idx_b])

    tokenized_inputs = tokenizer(
        sentences_a,
        sentences_b,
        max_length=max_length,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )

    next_sentence_labels = torch.tensor(nsp_labels, dtype=torch.long)
    return TensorDataset(
        tokenized_inputs.input_ids,
        tokenized_inputs.attention_mask,
        next_sentence_labels
    )


def run_inference(model, tokenizer, batch_size, mode):
    """
    Run a dummy BERT inference workload using the given model and tokenizer.
    Calculates average time per batch and throughput.
    """
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    model.to(device)
    model.eval()

    try:
        dataset = create_dummy_data(tokenizer, batch_size=batch_size, num_samples=100, max_length=128)
    except Exception:
        logger.exception("[ERROR] Failed to create dummy data.")
        raise

    dataloader = DataLoader(dataset, batch_size=batch_size)
    total_time = 0.0
    total_batches = len(dataloader)

    with torch.no_grad():
        for batch_idx, batch in enumerate(dataloader):
            try:
                inputs, masks, next_sentence_labels = batch
                inputs, masks, next_sentence_labels = (
                    inputs.to(device),
                    masks.to(device),
                    next_sentence_labels.to(device),
                )

                start_time = time.time()
                _ = model(
                    input_ids=inputs,
                    attention_mask=masks,
                    next_sentence_label=next_sentence_labels
                )
                end_time = time.time()
            except Exception:
                logger.exception(f"[ERROR] Inference failed on batch {batch_idx}.")
                raise

            total_time += (end_time - start_time)

    if total_time == 0.0:
        avg_time_per_batch = float('inf')
        throughput = 0.0
    else:
        avg_time_per_batch = total_time / total_batches
        throughput = (total_batches * batch_size) / total_time

    logger.info(
        "[BERT_INFERENCE_METRICS] "
        f"mode={mode} "
        f"avg_time_per_batch={avg_time_per_batch:.6f} "
        f"throughput_samples_per_sec={throughput:.6f}"
    )


def main():
    """
    Main entry point. Checks for GPU availability, determines inference mode,
    sets batch size, and runs inference. Logs throughput and timing stats.
    """
    if not torch.cuda.is_available():
        logger.error("[ERROR] GPU is not available. Exiting.")
        sys.exit(1)

    num_gpus = torch.cuda.device_count()
    logger.info(f"[INFO] Found {num_gpus} GPU(s). GPU is available.")

    mode = os.environ.get("INFERENCE_MODE", "throughput").lower()
    if mode not in ["throughput", "latency"]:
        logger.warning(
            f"[WARNING] Unrecognized INFERENCE_MODE '{mode}'. "
            "Falling back to 'throughput'."
        )
        mode = "throughput"

    batch_size = 1 if mode == "latency" else 8
    logger.info(f"[INFO] Running inference in {mode} mode with batch size {batch_size}.")

    try:
        tokenizer = BertTokenizer.from_pretrained("bert-base-uncased")
        model = BertForPreTraining.from_pretrained("bert-base-uncased")
    except Exception:
        logger.exception("[ERROR] Failed to load model/tokenizer. Exiting.")
        sys.exit(1)

    run_inference(model, tokenizer, batch_size, mode)


if __name__ == "__main__":
    main()
