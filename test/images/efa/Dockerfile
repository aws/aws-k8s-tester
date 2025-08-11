FROM public.ecr.aws/amazonlinux/amazonlinux:2023

ARG EFA_BIN_PATH="/opt/amazon/efa/bin"

RUN dnf -y swap gnupg2-minimal gnupg2 && \
    dnf install -y \
    gcc gcc-c++ make \  
    ca-certificates \
    cmake \
    emacs \
    git \
    jq \
    wget \
    unzip \
    vim \
    zlib-devel \      
    openssl \
    openssl-devel \    
    sqlite-devel \   
    gdbm-devel \      
    glibc-devel \     
    bzip2-devel \     
    ncurses-devel \    
    tk-devel \        
    libffi-devel \     
    libcap-devel \  
    tar \
    gnupg2 

ENV PATH="$PATH:$EFA_BIN_PATH"

RUN cd $HOME \
    && curl -O https://efa-installer.amazonaws.com/aws-efa-installer-latest.tar.gz \
    && wget https://efa-installer.amazonaws.com/aws-efa-installer.key && gpg --import aws-efa-installer.key \
    && cat aws-efa-installer.key | gpg --fingerprint \
    && wget https://efa-installer.amazonaws.com/aws-efa-installer-latest.tar.gz.sig && gpg --verify ./aws-efa-installer-latest.tar.gz.sig \
    && tar -xf aws-efa-installer-latest.tar.gz \
    && cd aws-efa-installer \
    && ./efa_installer.sh -y -d --skip-kmod --skip-limit-conf --no-verify \
    # TODO: remove this in favor of letting the efa installer add it if that ever becomes an option.
    # At the moment, this is only installed if omitting --no-verify, which would require
    # building in a context with EFA available
    && install -T -m 0755 efa_test.sh "${EFA_BIN_PATH}/efa_test.sh" \
    && cd $HOME \
    && rm -rf aws-efa-installer

RUN dnf clean all

RUN INSTALL_DIR=$(mktemp -d) && \
    cd $INSTALL_DIR && \
    curl "https://awscli.amazonaws.com/awscli-exe-linux-$(uname -m).zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install  && \
    cd && \ 
    rm -rf $INSTALL_DIR

COPY test/images/efa/scripts ./scripts

RUN chmod -R +x ./scripts