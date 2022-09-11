# tpmhbs

A tool for projecting the performance of Hash-Based Signature schemes on
various TPMs.

## Caveats

This tool depends on some extremely rough estimates of the hashing/KDF work
required for LMS and XMSS. These estimates are intended for ballpark/relative
estimation of the feasibility of the various NIST-approved parameter sets on
current TPM hardware. These estimates may get better over time.

## How to build this tool

```console
git clone https://github.com/chrisfenner/tpmhbs
cd tpmhbs
go build .
```

## How to use this tool

On a system with a TPM:

```console
tpmhbs [--sort_by={keygen, signing, size, name}]
```

On Linux, this command may need to be run as sudo.
