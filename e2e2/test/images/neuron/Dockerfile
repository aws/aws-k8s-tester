FROM public.ecr.aws/docker/library/ubuntu:20.04

# Neuron repos
ARG APT_REPO=https://apt.repos.neuron.amazonaws.com
ARG PIP_REPO=https://pip.repos.neuron.amazonaws.com

ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12
ARG PIP="${PYTHON} -m pip"
ARG OMPI_VERSION=4.1.5

# This arg required to stop docker build waiting for region configuration while installing tz data from ubuntu 20
ARG DEBIAN_FRONTEND=noninteractive

# Python won’t try to write .pyc or .pyo files on the import of source modules
# Force stdin, stdout and stderr to be totally unbuffered. Good for logging
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PYTHONIOENCODING=UTF-8
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/aws/neuron/lib"
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/amazon/efa/lib"
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/amazon/efa/lib64"
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/amazon/openmpi/lib64"
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/usr/local/lib"
ENV PATH /opt/aws/neuron/bin/:$PATH
# ENV SAGEMAKER_TRAINING_MODULE=sagemaker_pytorch_container.training:main
ENV DGLBACKEND=pytorch

RUN apt-get update \
 && apt-get upgrade -y \
 && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    curl \
    emacs \
    git \
    jq \
    libopencv-dev \
    software-properties-common \
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
    libcap-dev \
    gnupg2 \
    gpg-agent \
 && rm -rf /var/lib/apt/lists/* \
 && apt-get clean

RUN echo "deb https://apt.repos.neuron.amazonaws.com focal main" > /etc/apt/sources.list.d/neuron.list
RUN wget -qO - https://apt.repos.neuron.amazonaws.com/GPG-PUB-KEY-AMAZON-AWS-NEURON.PUB | apt-key add -

# install Python
RUN wget -q https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz \
 && tar -xzf Python-$PYTHON_VERSION.tgz \
 && cd Python-$PYTHON_VERSION \
 && ./configure --enable-shared --prefix=/usr/local \
 && make -j $(nproc) && make install \
 && cd .. && rm -rf ../Python-$PYTHON_VERSION* \
 && ln -s /usr/local/bin/pip3 /usr/bin/pip \
 && ln -s /usr/local/bin/$PYTHON /usr/local/bin/python \
 && ${PIP} --no-cache-dir install --upgrade \
    pip \
    setuptools

# EFA Installer does apt get. Make sure to run apt update before that
RUN apt-get update
RUN cd $HOME \
 && curl -O https://efa-installer.amazonaws.com/aws-efa-installer-latest.tar.gz \
 && wget https://efa-installer.amazonaws.com/aws-efa-installer.key && gpg --import aws-efa-installer.key \
 && cat aws-efa-installer.key | gpg --fingerprint \
 && wget https://efa-installer.amazonaws.com/aws-efa-installer-latest.tar.gz.sig && gpg --verify ./aws-efa-installer-latest.tar.gz.sig \
 && tar -xf aws-efa-installer-latest.tar.gz \
 && cd aws-efa-installer \
 && ./efa_installer.sh -y -g -d --skip-kmod --skip-limit-conf --no-verify \
 && cd $HOME


# Neuron system packages (minus driver)
RUN echo "deb $APT_REPO focal main" > /etc/apt/sources.list.d/neuron.list \
    && wget -qO - $APT_REPO/GPG-PUB-KEY-AMAZON-AWS-NEURON.PUB | apt-key add - \
    && apt-get update \
    && apt-get install -y aws-neuronx-tools aws-neuronx-collectives aws-neuronx-runtime-lib \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /tmp/tmp* \
    && apt-get clean

RUN curl https://bootstrap.pypa.io/get-pip.py -o get-pip.py && $PYTHON get-pip.py && $PIP install --upgrade pip
RUN mkdir -p /etc/pki/tls/certs && cp /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt

# PyTorch Neuron packages (2.12)
RUN $PIP config set global.extra-index-url $PIP_REPO \
    && $PIP install --force-reinstall torch-neuronx neuronx-cc neuronx_distributed \
    && $PIP install --no-cache-dir -U python-etcd \
    && $PIP install torchvision==0.16.* packaging \
    && rm -fr /root/.cache/

# Install packages and configure SSH for MPI operator in k8s
RUN apt-get update && apt-get install -y openmpi-bin openssh-server \
    && mkdir -p /var/run/sshd \
    && echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config \
    && echo "    StrictHostKeyChecking no" >> /etc/ssh/ssh_config \
    && sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /tmp/tmp* \
    && apt-get clean

# Add Neuron tools to path
RUN echo "export PATH=/opt/aws/neuron/bin:\$PATH" >> /root/.bashrc \
    && echo "export TERM=screen" >> /root/.bashrc

# Clean up after apt update
RUN rm -rf /var/lib/apt/lists/* \
 && rm -rf /tmp/tmp* \
 && apt-get clean

WORKDIR /app

COPY e2e2/test/images/neuron/unit-test /app/unit-test

COPY e2e2/test/images/neuron/e2e-test /app/e2e-test

# Install any needed packages for multi-node-test
RUN python -m pip install --upgrade pip && \
    pip install --no-cache-dir -r e2e-test/requirements.txt