
variable "image_home_dir" {
  type    = string
  default = "/home/pi"
}

variable "ssh_authorized_keys_file" {
  type    = string
  default = "${env("HOME")}/.ssh/id_rsa.pub"
}

variable "wifi_country" {
  type    = string
  default = "DE"
}

variable "wifi_password" {
  type    = string
  default = ""
}

variable "wifi_ssid" {
  type    = string
  default = ""
}

source "arm-image" "raspios" {
  iso_checksum      = "58a3ec57402c86332e67789a6b8f149aeeb4e7bb0a16c9388a66ea6e07012e45"
  iso_url           = "https://downloads.raspberrypi.org/raspios_lite_arm64/images/raspios_lite_arm64-2024-03-15/2024-03-15-raspios-bookworm-arm64-lite.img.xz"
  qemu_binary       = "qemu-aarch64-static"
  target_image_size = 4294967296
}

build {
  sources = ["source.arm-image.raspios"]

  provisioner "shell" {
    # Enable memory cgroup support
    # (Original contents: console=serial0,115200 console=tty1 root=PARTUUID=610a07ef-02 rootfstype=ext4 fsck.repair=yes rootwait)
    inline = [
      "sed -Ei 's/$/ cgroup_enable=cpuset cgroup_memory=1 cgroup_enable=memory/' /boot/cmdline.txt",
      "echo 'cgroup /sys/fs/cgroup cgroup defaults 0 0' >> /etc/fstab"
    ]
  }

  provisioner "shell" {
    # Configure the wifi client
    inline = [
      "mv /etc/wpa_supplicant/wpa_supplicant.conf /boot/wpa_supplicant.conf",
      "sed -Ei '/^country=/d' /boot/wpa_supplicant.conf",
      "echo 'country=${var.wifi_country}' >> /boot/wpa_supplicant.conf",
      "[ -z \"${var.wifi_ssid}\" ] || [ -z \"${var.wifi_password}\" ] || (wpa_passphrase \"${var.wifi_ssid}\" \"${var.wifi_password}\" | sed -e 's/#.*$//' -e '/^$/d' >> /boot/wpa_supplicant.conf && echo WLAN enabled)",
      "echo 1 > '/var/lib/systemd/rfkill/platform-soc:bluetooth'"
    ]
  }

  provisioner "shell" {
    inline = ["mkdir ${var.image_home_dir}/.ssh"]
  }

  provisioner "file" {
    # Register the build host/user's SSH key with the image
    destination = "${var.image_home_dir}/.ssh/authorized_keys"
    source      = "${var.ssh_authorized_keys_file}"
  }

  provisioner "shell" {
    # Enable the SSH server
    inline = [
      "chmod 644 ${var.image_home_dir}/.ssh/authorized_keys",
      "touch /boot/ssh"
    ]
  }

  provisioner "shell" {
    # Upgrade and install optional client tools.
    inline = [
      "apt-get update",
      "apt-get upgrade -y",
      "apt-get install -y kubernetes-client vim git",
      "echo 'set mouse=' > /root/.vimrc",
      "cp /root/.vimrc '${var.image_home_dir}/.vimrc'",
      "chown pi:pi '${var.image_home_dir}/.vimrc'",
    ]
  }

  provisioner "shell" {
    # Install docker
    inline = [
      "curl -fsSL https://get.docker.com | sh",
      "usermod -a -G docker pi",
      # Configure docker to use cgroup driver "cgroupfs" instead of "systemd" since k3s does not support systemd.
      # This is due to static linking, see https://github.com/k3s-io/k3s/issues/797
      "mkdir -p /etc/docker",
      "printf '{\n  \"exec-opts\": [\"native.cgroupdriver=cgroupfs\"]\n}' > /etc/docker/daemon.json"
    ]
  }

  provisioner "shell" {
    # Write kubemate version file.
    inline = [
      "mkdir /etc/kubemate",
      "echo latest > /etc/kubemate/version",
    ]
  }

  provisioner "file" {
    # Add the hostnamegen systemd unit that generates /etc/hostname on first boot
    destination = "/lib/systemd/system/hostnamegen.service"
    source      = "./packer/systemd/hostnamegen.service"
  }

  provisioner "file" {
    # Add script to generate hostname.
    destination = "/usr/local/sbin/hostnamegen.sh"
    source      = "./packer/hostnamegen.sh"
  }

  provisioner "file" {
    # Add the kubemate systemd unit
    destination = "/lib/systemd/system/kubemate.service"
    source      = "./packer/systemd/kubemate.service"
  }

  provisioner "file" {
    # Add script to kill docker pods
    destination = "/usr/local/sbin/kill-docker-pods.sh"
    source      = "./kill-docker-pods.sh"
  }

  provisioner "file" {
    # Add script to stop kubemate and delete its persistent state.
    # This script must be run before updates.
    destination = "/usr/local/sbin/kubemate-clear.sh"
    source      = "./kubemate-clear.sh"
  }

  provisioner "shell" {
    # Grant the default user administrative Kubernetes access
    inline = [
      "mkdir ${var.image_home_dir}/.kube",
      "ln -s /etc/kubemate/kubeconfig.yaml ${var.image_home_dir}/.kube/config",
      "mkdir -p /etc/kubemate",
      "touch /etc/kubemate/kubeconfig.yaml",
      "chown root:1000 /etc/kubemate/kubeconfig.yaml",
    ]
  }

  provisioner "shell" {
    # Disable default wpa_supplicant (to let kubemate manage the wifi)
    inline = [
      "systemctl mask wpa_supplicant.service",
      "printf '[keyfile]\nunmanaged-devices=interface-name:wlan0\n' > /etc/NetworkManager/conf.d/99-unmanaged-devices.conf",
    ]
  }

  provisioner "shell" {
    # Enable the previously added systemd units and remove /etc/hostname
    inline = [
      "systemctl daemon-reload",
      "systemctl enable kubemate.service hostnamegen.service",
      "rm -f /etc/hostname",
    ]
  }
}
