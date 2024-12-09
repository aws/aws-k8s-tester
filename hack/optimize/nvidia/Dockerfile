# Use NVIDIA CUDA base image
FROM nvidia/cuda:12.1.1-cudnn8-runtime-ubuntu22.04

# Install dependencies
RUN apt-get update && apt-get install -y \
    python3.10 \
    python3.10-dev \
    python3.10-distutils \
    curl && \
    rm -rf /var/lib/apt/lists/*

# Install pip
RUN curl -sS https://bootstrap.pypa.io/get-pip.py | python3.10

# Set up Python environment and install torch with CUDA support
WORKDIR /app
RUN python3.10 -m pip install --upgrade pip && \
    python3.10 -m pip install --no-cache-dir torch==2.0.1+cu118 -f https://download.pytorch.org/whl/torch_stable.html

# Install additional dependencies
COPY requirements.txt .
RUN python3.10 -m pip install --no-cache-dir -r requirements.txt

# Copy scripts into the container
COPY train_bert_nvidia.py /app/train_bert_nvidia.py
COPY infer_bert_nvidia.py /app/infer_bert_nvidia.py

