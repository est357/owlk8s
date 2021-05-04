#!/bin/bash

if ! [ "${PWD##*/}" == "bpf_prog" ]
then
  cd metrics/bpf_prog
fi

if [ "`docker images | grep owlk8s-build-bpf | wc -l`" -eq 0 ]
then
    docker build -t owlk8s-build-bpf .
fi
if [ "`docker ps | grep owlk8s-build-bpf | wc -l`" -eq "0" ]
then
    docker run --name owlk8s-build-bpf -d owlk8s-build-bpf
fi
docker exec owlk8s-build-bpf /bin/bash -c "[ -d /root/bpf_prog ] && rm -rf /root/bpf_prog"
docker cp -a ../bpf_prog owlk8s-build-bpf:/root/bpf_prog
docker exec owlk8s-build-bpf /bin/bash -c "cd /root/bpf_prog/;clang -D__KERNEL__ -D__ASM_SYSREG_H -Wno-unused-value -Wno-pointer-sign -Wno-compare-distinct-pointer-types -Wunused -Wall -Werror -O2 -g -target bpf -c http.c -o http.o"
docker cp owlk8s-build-bpf:/root/bpf_prog/http.o .
go run main.go
git add ../eBPFprog.go
git commit  -m 'Change eBPG prog'
git push
