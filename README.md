# kubemate

An experimental k3s-based Kubernetes distribution to create local clusters on IoT devices.

## Build from source

This section describes how to build and test kubemate from source.

### Prerequisites

The following tools need to be installed on your host:

* make
* docker 20+

### Build the container image

To build a container image for the linux/amd64 platform using docker, run:
```sh
make container
```

### Test the container locally

To test kubemate on your local machine, you can run it within a container within the host network using `make run` and browse [`https://localhost:8080`](https://localhost:8443).  

To test the clustering locally, run a second kubemate container using `make run-other` within another terminal.
To get its IP address, run within another terminal:
```sh
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' kubemate2
```

Now browse the 2nd container's web UI at `https://<CONTAINER_IP>:8443`.
Within the device list you should be able to see the first container and make the 2nd kubemate container join it as agent.  

_Please note that within this local test setup only the 2nd container (that is within a docker network) can find the 1st container (that is within the host network) since discovery works using mDNS but docker propagates only mDNS broadcasts from the host into the container networks - not the other way around._

#### Docker configuration on the host

To make kubemate work well with the docker installation on your host, you have to configure docker to use the `cgroupfs` driver, e.g. by configuring `/etc/docker/daemon.json` as follows:
```json
{
	"exec-opts": ["native.cgroupdriver=cgroupfs"]
}
```

#### Networking

To make sure pod networking is working properly, use nf_tables instead of iptables legacy on your host.

#### Upgrades

To apply a major version upgrade, uninstall/clear the entire state of the existing installation before launching the new version.

#### Clear docker pods

To kill and remove all docker containers that originate from Kubernetes as well as their volumes, run [`kill-docker-pods.sh`](./kill-docker-pods.sh).

#### Delete the state

Stop kubemate, delete all docker containers, delete the persistent state: [`kubemate-clear.sh`](./kubemate-clear.sh).

### Prepare a Raspberry Pi OS image

To build a complete SD card image to run kubemate on a Raspberry Pi on top of the Raspberry Pi OS, run
```sh
make raspios-image
```

#### Flash the image to an SD card

You can write a previously built image to an SD card as follows:

```sh
TARGET_DEVICE=/dev/sdX
sudo umount ${TARGET_DEVICE}* || true
sudo dd bs=4M if=./output-raspios/image of="$TARGET_DEVICE"
sync
```

**ATTENTION:** Please replace `/dev/sdX` carefully with the path to the device you want to write the image to - specifying the wrong device can cause data loss!
To find the correct device path, you can use `lsblk`.
