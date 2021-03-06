
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
  iso_checksum      = "35f1d2f4105e01f4ca888ab4ced6912411e82a2539c53c9e4e6b795f25275a1f"
  iso_url           = "https://downloads.raspberrypi.org/raspios_lite_arm64/images/raspios_lite_arm64-2022-04-07/2022-04-04-raspios-bullseye-arm64-lite.img.xz"
  qemu_binary       = "qemu-aarch64-static"
  target_image_size = 4294967296
}

build {
  sources = ["source.arm-image.raspios"]

  provisioner "shell" {
	# Enable memory cgroup support.
	# (Original contents: console=serial0,115200 console=tty1 root=PARTUUID=610a07ef-02 rootfstype=ext4 fsck.repair=yes rootwait)
    inline = [
      "sed -Ei 's/$/ cgroup_memory=1 cgroup_enable=memory/' /boot/cmdline.txt"
    ]
  }

  provisioner "shell" {
	# Configure the wifi client.
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
	# Register the build host/user's SSH key with the image.
    destination = "${var.image_home_dir}/.ssh/authorized_keys"
    source      = "${var.ssh_authorized_keys_file}"
  }

  provisioner "shell" {
	# Enable the SSH server.
    inline = [
      "chmod 644 ${var.image_home_dir}/.ssh/authorized_keys",
      "touch /boot/ssh"
    ]
  }

  provisioner "shell" {
	# Configure the hostname.
    inline = [
      "echo ${var.hostname} > /etc/hostname"
    ]
  }

  provisioner "shell" {
	# Install docker.
    inline = [
      "curl -fsSL https://get.docker.com | sh",
      "usermod -a -G docker pi"
    ]
  }

  provisioner "file" {
	# Add the kubemate systemd unit.
    destination = "/lib/systemd/system/kubemate.service"
    source      = "./packer/systemd/kubemate.service"
  }

  provisioner "file" {
	# Add kubectl support to the host
    destination = "/usr/local/bin/kubectl"
    source      = "./packer/kubectl"
  }

  provisioner "shell" {
	# Grant the default user administrative Kubernetes access.
    inline = [
      "mkdir ${var.image_home_dir}/.kube",
      "ln -s /etc/kubemate/kubeconfig.yaml ${var.image_home_dir}/.kube/config",
      "mkdir -p /etc/kubemate",
      "touch /etc/kubemate/kubeconfig.yaml",
      "chown root:1000 /etc/kubemate/kubeconfig.yaml"
    ]
  }

  provisioner "shell" {
	# Enable the kubemate systemd unit.
    inline = [
      "systemctl daemon-reload",
      "systemctl enable kubemate.service",
      #"bash <(curl -L https://github.com/balena-io/wifi-connect/raw/v4.4.6/scripts/raspbian-install.sh)"
    ]
  }
}
