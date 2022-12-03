
variable "hostname" {
  type    = string
  default = "raspberrypi"
}

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
  iso_checksum      = "72c773781a0a57160eb3fa8bb2a927642fe60c3af62bc980827057bcecb7b98b"
  iso_url           = "https://downloads.raspberrypi.org/raspios_lite_arm64/images/raspios_lite_arm64-2022-09-26/2022-09-22-raspios-bullseye-arm64-lite.img.xz"
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
    # Configure the hostname
    inline = [
      "echo ${var.hostname} > /etc/hostname"
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
    # Add kubectl support to the host
    destination = "/usr/local/sbin/hostnamegen.sh"
    source      = "./packer/hostnamegen.sh"
  }

  provisioner "file" {
    # Add the kubemate systemd unit
    destination = "/lib/systemd/system/kubemate.service"
    source      = "./packer/systemd/kubemate.service"
  }

  provisioner "file" {
    # Add kubernetes client CLI support to the host
    destination = "/usr/local/bin/kubectl"
    source      = "./packer/kubectl"
  }

  provisioner "shell" {
    # Grant the default user administrative Kubernetes access
    inline = [
      "mkdir ${var.image_home_dir}/.kube",
      "ln -s /etc/kubemate/kubeconfig.yaml ${var.image_home_dir}/.kube/config",
      "mkdir -p /etc/kubemate",
      "touch /etc/kubemate/kubeconfig.yaml",
      "chown root:1000 /etc/kubemate/kubeconfig.yaml",
      "chmod +rx /usr/local/bin/kubectl"
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
