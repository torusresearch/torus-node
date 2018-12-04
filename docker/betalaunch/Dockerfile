FROM ubuntu:18.04

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && apt-get install wget && apt-get install git && apt-get install make && apt install build-essential && \
    wget https://dl.google.com/go/go1.11.2.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.11.2.linux-amd64.tar.gz && \
    echo "export PATH=$PATH:/usr/local/go/bin" >> $HOME/.profile && echo "export GOPATH=$HOME/go" >> $HOME/.profile && rm go1.11.2.linux-amd64.tar.gz && source $HOME/.profile && \
    cd $GOPATH && go get github.com/YZhenY/torus && mkdir $GOPATH/src/github.com/tendermint && cd $GOPATH/src/github.com/tendermint && \
    echo "export PATH=$PATH:$GOPATH/bin" >> $HOME/.profile && source $HOME/.profile && \
    # wget https://raw.githubusercontent.com/golang/dep/master/install.sh && chmod +x ./install.sh && ./install.sh && \
    git clone https://github.com/YZhenY/tendermint && cd $GOPATH/src/github.com/tendermint/tendermint && \
    make get_tools && make get_vendor_deps && \
    apt-get install libsnappy-dev && \
    wget https://github.com/google/leveldb/archive/v1.20.tar.gz && \
    tar -zxvf v1.20.tar.gz && \
    cd leveldb-1.20/ && \
    make && \
    cp -r out-static/lib* out-shared/lib* /usr/local/lib/ && \
    cd include/ && \
    cp -r leveldb /usr/local/include/ && \
    ldconfig && \
    rm -f v1.20.tar.gz && cd $GOPATH/src/github.com/YZhenY/torus


    
    
