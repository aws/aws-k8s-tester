FROM public.ecr.aws/docker/library/ubuntu:22.04

###############################################################################
# 0) Arguments and environment
###############################################################################
ARG DEBIAN_FRONTEND=noninteractive

# Neuron SDK component versions (pin these precisely)
ARG NEURONX_CC_VERSION=2.16.345.0
ARG NEURONX_TOOLS_VERSION=2.20.204.0
ARG NEURONX_FRAMEWORK_VERSION=2.5.1.2.4.0
ARG NEURONX_RUNTIME_LIB_VERSION=2.23.110.0-9b5179492
ARG NEURONX_COLLECTIVES_LIB_VERSION=2.23.133.0-3e70920f2

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
     sudo \
     && rm -rf /var/lib/apt/lists/* \
     && apt-get clean

# Create a non-root 'ubuntu' user with password 'password'
RUN useradd -ms /bin/bash ubuntu \
     && echo 'ubuntu:password' | chpasswd \
     && usermod -aG sudo ubuntu

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
RUN sed -i -e 's/#   StrictHostKeyChecking ask/    StrictHostKeyChecking accept-new/g' /etc/ssh/ssh_config

# Give 'ubuntu' a known password so 'sudo -S' works:
RUN echo 'ubuntu:password' | chpasswd

# Switch to 'ubuntu' user (non-root).
WORKDIR /home/ubuntu
USER ubuntu

# Create SSH keypair in /home/ubuntu/.ssh
RUN mkdir -p /home/ubuntu/.ssh \
     && ssh-keygen -t rsa -f /home/ubuntu/.ssh/id_rsa -N '' \
     && echo password | sudo -S chmod 600 /home/ubuntu/.ssh/id_rsa \
     && echo password | sudo -S chmod 600 /home/ubuntu/.ssh/id_rsa.pub \
     && echo password | sudo -S cat /home/ubuntu/.ssh/id_rsa.pub >> /home/ubuntu/.ssh/authorized_keys

# Copy training script (optional)
COPY train.py /home/ubuntu/

# Expose SSH port and run sshd
EXPOSE 22
CMD ["/usr/sbin/sshd","-D"]
