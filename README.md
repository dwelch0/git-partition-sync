# Git Partition Sync
Keeping private repositories in sync across a network partition is a common problem that typically requires (recurring) manual intervention.

**Git Partition Sync** aims to automate this process by utilizing an s3 bucket and two containerized applications: `producer` and `consumer`.

![diagram](diagram.png)

## Demo
TDB

## Components

### Producer
`producer` is tasked with cloning, tarring, encrypting, and uploading specified GitLab repositories to s3. `producer` formats the keys of uploaded objects as base64 encoded json for easy processing by `consumer`.

Refer to [producer's README](/producer/README.md) for further details.

### Consumer
`consumer` is tasked with downloading, decrypting, untarring, and pushing GitLab repositories to desired targets. 

Refer to [consumer's README](/consumer/README.md) for further details.
