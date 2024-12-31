FROM public.ecr.aws/docker/library/ubuntu:20.04

###############################################################################
# 0) Arguments and environment
###############################################################################
ARG DEBIAN_FRONTEND=noninteractive

# Neuron SDK components version numbers
ARG NEURONX_DISTRIBUTED_VERSION=0.9.0
ARG NEURONX_CC_VERSION=2.15.141.0 
ARG NEURONX_FRAMEWORK_VERSION=2.1.2.2.3.1
ARG NEURONX_COLLECTIVES_LIB_VERSION=2.22.26.0-17a033bc8
ARG NEURONX_RUNTIME_LIB_VERSION=2.22.14.0-6e27b8d5b
ARG NEURONX_TOOLS_VERSION=2.19.0.0

# Python
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12
ARG PIP=pip3

# PyTorch
ARG PYTORCH_GIT_BRANCH="v2.1.0"

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONIOENCODING=UTF-8 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/aws/neuron/lib"
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/amazon/efa/lib"
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/amazon/efa/lib64"
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/usr/local/lib"
ENV PATH /opt/aws/neuron/bin/:$PATH

###############################################################################
# 1) Base system packages, user setup
###############################################################################
RUN apt-get update \
 && apt-get upgrade -y \
 && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    emacs \
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
      aws-neuronx-tools=$NEURONX_TOOLS_VERSION \
      aws-neuronx-collectives=$NEURONX_COLLECTIVES_LIB_VERSION \
      aws-neuronx-runtime-lib=$NEURONX_RUNTIME_LIB_VERSION \
 && rm -rf /var/lib/apt/lists/* \
 && apt-get clean


###############################################################################
# 3) EFA installer (provides /opt/amazon/efa, /opt/amazon/openmpi, etc.)
###############################################################################
RUN apt-get update
RUN cd $HOME \
 && curl -O https://efa-installer.amazonaws.com/aws-efa-installer-latest.tar.gz \
 && wget https://efa-installer.amazonaws.com/aws-efa-installer.key && gpg --import aws-efa-installer.key \
 && cat aws-efa-installer.key | gpg --fingerprint \
 && wget https://efa-installer.amazonaws.com/aws-efa-installer-latest.tar.gz.sig && gpg --verify ./aws-efa-installer-latest.tar.gz.sig \
 && tar -xf aws-efa-installer-latest.tar.gz \
 && cd aws-efa-installer \
 && ./efa_installer.sh -y -g -d --skip-kmod --skip-limit-conf --no-verify \
 && cd $HOME \
 && rm -rf aws-efa-installer* \
 && rm -rf /var/lib/apt/lists/* \
 && apt-get clean

ENV PATH="/opt/amazon/openmpi/bin:${PATH}"
ENV LD_LIBRARY_PATH="/opt/amazon/openmpi/lib64:${LD_LIBRARY_PATH}"

###############################################################################
# 4) Python 3.10 from source
###############################################################################
RUN wget -q https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz \
 && tar -xzf Python-$PYTHON_VERSION.tgz \
 && cd Python-$PYTHON_VERSION \
 && ./configure --enable-shared --prefix=/usr/local \
 && make -j $(nproc) && make install \
 && cd .. && rm -rf Python-$PYTHON_VERSION* \
 && ln -s /usr/local/bin/pip3 /usr/bin/pip \
 && ln -s /usr/local/bin/$PYTHON /usr/local/bin/python \
 && pip --no-cache-dir install --upgrade pip setuptools

###############################################################################
# 5) Base Python packages (Neuron, others)
###############################################################################
RUN pip install --no-cache-dir -U \
    "bokeh>=2.3,<3" \
    "awscli<2" \
    scipy \
    click \
    "cryptography" \
    psutil==5.6.7 \
    dataset \
    transformers==4.36.2 \
    Pillow

RUN mkdir -p /etc/pki/tls/certs && cp /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt

# Use official Neuron + torch/xla packages from the Neuron PyPI
RUN pip config set global.extra-index-url https://pip.repos.neuron.amazonaws.com \
 && pip install --force-reinstall torch-neuronx==$NEURONX_FRAMEWORK_VERSION \
 && pip install --force-reinstall neuronx-cc==$NEURONX_CC_VERSION \
 && pip install --force-reinstall --no-deps neuronx-distributed==$NEURONX_DISTRIBUTED_VERSION

RUN pip install --no-cache-dir -U \
    "attrs<24,>=23.1.0" \
    "protobuf>=3.18.3,<=3.20.3" \
    "docutils>=0.10,<0.17" \
    "rsa<4.8,>=3.1.2" \
    "urllib3>=1.26.0,<1.27"

###############################################################################
# 6) TorchVision (no-deps)
###############################################################################
RUN pip install --no-cache-dir --no-deps -U torchvision==0.16.*

###############################################################################
# 7) OSS compliance steps 
###############################################################################
RUN HOME_DIR=/root \
 && curl -o ${HOME_DIR}/oss_compliance.zip https://aws-dlinfra-utilities.s3.amazonaws.com/oss_compliance.zip \
 && unzip ${HOME_DIR}/oss_compliance.zip -d ${HOME_DIR}/ \
 && cp ${HOME_DIR}/oss_compliance/test/testOSSCompliance /usr/local/bin/testOSSCompliance \
 && chmod +x /usr/local/bin/testOSSCompliance \
 && chmod +x ${HOME_DIR}/oss_compliance/generate_oss_compliance.sh \
 && ${HOME_DIR}/oss_compliance/generate_oss_compliance.sh ${HOME_DIR} python3.10 \
 && rm -rf ${HOME_DIR}/oss_compliance*

RUN curl -o /license.txt https://aws-dlc-licenses.s3.amazonaws.com/pytorch-2.1/license.txt

###############################################################################
# 8) SSH, finalize
###############################################################################
# 1) Configure SSH so we don't prompt on first connect:
RUN sed -i -e 's/#   StrictHostKeyChecking ask/    StrictHostKeyChecking accept-new/g' /etc/ssh/ssh_config

# 2) Give the "ubuntu" user a known password so sudo -S works:
RUN echo 'ubuntu:password' | chpasswd

# 3) Switch to the "ubuntu" user (non-root).
WORKDIR /home/ubuntu
USER ubuntu

# 4) Create SSH keypair in /home/ubuntu/.ssh
RUN mkdir -p /home/ubuntu/.ssh && \
    ssh-keygen -t rsa -f /home/ubuntu/.ssh/id_rsa -N '' && \
    echo password | sudo -S chmod 600 /home/ubuntu/.ssh/id_rsa && \
    echo password | sudo -S chmod 600 /home/ubuntu/.ssh/id_rsa.pub && \
    echo password | sudo -S cat /home/ubuntu/.ssh/id_rsa.pub >> /home/ubuntu/.ssh/authorized_keys

# 5) Copy your training script or other files
COPY train.py /home/ubuntu/

# 6) Expose port 22 and run sshd by default
EXPOSE 22
CMD ["/usr/sbin/sshd","-D"]
