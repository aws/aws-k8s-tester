# Use the NVIDIA CUDA runtime as a parent image
FROM nvidia/cuda:12.5.0-devel-ubuntu22.04

# Set environment variable to disable interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install Python 3.11
RUN apt-get update && apt-get install -y \
    software-properties-common && \
    add-apt-repository ppa:deadsnakes/ppa && \
    apt-get update && \
    apt-get install -y \
    python3.11 \
    python3.11-dev \
    python3.11-distutils \
    python3-pip && \
    rm -rf /var/lib/apt/lists/*

# Create a symbolic link to use python3.11 as python
RUN ln -sf /usr/bin/python3.11 /usr/bin/python

# Set the working directory in the container
WORKDIR /app

# Copy only the necessary files into the container at /app
COPY infer.py /app/
COPY requirements.txt /app/

# Install any needed packages specified in requirements.txt
RUN python -m pip install --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt
