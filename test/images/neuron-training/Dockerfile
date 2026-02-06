FROM public.ecr.aws/docker/library/ubuntu:22.04

###############################################################################
# 0) Arguments and environment
###############################################################################
ARG DEBIAN_FRONTEND=noninteractive

# Neuron SDK component versions (pin these precisely)
# https://github.com/aws-neuron/deep-learning-containers/blob/main/docker/pytorch/training/2.5.1/Dockerfile.neuronx
ARG NEURONX_CC_VERSION=2.22.12471.0+b4a00d10
ARG NEURONX_FRAMEWORK_VERSION=2.9.0.2.11.19912+e48cd891
ARG NEURONX_COLLECTIVES_LIB_VERSION=2.29.41.0-681fef5f5
ARG NEURONX_RUNTIME_LIB_VERSION=2.29.40.0-f954cd7a5
ARG NEURONX_TOOLS_VERSION=2.27.33.0-5d9c0b901

# Python
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12

ENV PYTHONDONTWRITEBYTECODE=1 \
     PYTHONUNBUFFERED=1 \
     PYTHONIOENCODING=UTF-8 \
     LANG=C.UTF-8 \
     LC_ALL=C.UTF-8

# Extend library paths for Neuron & EFA
ENV LD_LIBRARY_PATH="/opt/aws/neuron/lib:/opt/amazon/efa/lib:/opt/amazon/efa/lib64:/usr/local/lib:${LD_LIBRARY_PATH}"
ENV PATH="/opt/aws/neuron/bin:${PATH}"

###############################################################################
# 1) Base system packages, user setup
###############################################################################
RUN apt-get update \
     && apt-get upgrade -y \
     && apt-get install -y --no-install-recommends \
     build-essential \
     ca-certificates \
     curl \
     git \
     jq \
     wget \
     unzip \
     vim \
     lcov \
     pkg-config \
     zlib1g-dev \
     openssl \
     libssl-dev \
     libsqlite3-dev \
     libgdbm-dev \
     libc6-dev \
     libbz2-dev \
     libncurses-dev \
     tk-dev \
     libffi-dev \
     gnupg2 \
     gpg-agent \
     openssh-server \ 
     openssh-client \
     sudo \
     && rm -rf /var/lib/apt/lists/* \
     && apt-get clean

###############################################################################
# 2) Neuron SDK
###############################################################################
RUN . /etc/os-release \
     && echo "deb https://apt.repos.neuron.amazonaws.com focal main" > /etc/apt/sources.list.d/neuron.list \
     && wget -qO - https://apt.repos.neuron.amazonaws.com/GPG-PUB-KEY-AMAZON-AWS-NEURON.PUB | apt-key add - \
     && apt-get update \
     && apt-get install -y \
     aws-neuronx-tools=${NEURONX_TOOLS_VERSION} \
     aws-neuronx-collectives=${NEURONX_COLLECTIVES_LIB_VERSION} \
     aws-neuronx-runtime-lib=${NEURONX_RUNTIME_LIB_VERSION} \
     && rm -rf /var/lib/apt/lists/* \
     && apt-get clean

###############################################################################
# 3) EFA installer (for MPI-based jobs)
###############################################################################
RUN apt-get update \
     && cd /tmp \
     && curl -O https://efa-installer.amazonaws.com/aws-efa-installer-latest.tar.gz \
     && wget https://efa-installer.amazonaws.com/aws-efa-installer.key \
     && gpg --import aws-efa-installer.key \
     && cat aws-efa-installer.key | gpg --fingerprint \
     && wget https://efa-installer.amazonaws.com/aws-efa-installer-latest.tar.gz.sig \
     && gpg --verify ./aws-efa-installer-latest.tar.gz.sig \
     && tar -xf aws-efa-installer-latest.tar.gz \
     && cd aws-efa-installer \
     && ./efa_installer.sh -y -g -d --skip-kmod --skip-limit-conf --no-verify \
     && cd /tmp \
     && rm -rf aws-efa-installer* \
     && rm -rf /var/lib/apt/lists/* \
     && apt-get clean

ENV PATH="/opt/amazon/openmpi/bin:${PATH}"
ENV LD_LIBRARY_PATH="/opt/amazon/openmpi/lib64:${LD_LIBRARY_PATH}"

###############################################################################
# 4) Python 3.10 from source
###############################################################################
RUN wget -q https://www.python.org/ftp/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tgz \
     && tar -xzf Python-${PYTHON_VERSION}.tgz \
     && cd Python-${PYTHON_VERSION} \
     && ./configure --enable-shared --prefix=/usr/local \
     && make -j $(nproc) && make install \
     && cd .. && rm -rf Python-${PYTHON_VERSION}* \
     && ln -s /usr/local/bin/pip3 /usr/bin/pip \
     && ln -s /usr/local/bin/${PYTHON} /usr/local/bin/python \
     && pip --no-cache-dir install --upgrade pip setuptools wheel

###############################################################################
# 5) Install pinned Python packages
###############################################################################
RUN pip config set global.extra-index-url https://pip.repos.neuron.amazonaws.com \
     && pip install --force-reinstall \
     "torch-neuronx==${NEURONX_FRAMEWORK_VERSION}" \
     "neuronx-cc==${NEURONX_CC_VERSION}" \
     "transformers==4.36.2"

###############################################################################
# 6) SSH and finalize
###############################################################################
# Configure SSH (auto-accept new host keys)
RUN mkdir -p /var/run/sshd && \
     sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
     echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
     sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config


WORKDIR /app/
COPY train.py /app/
