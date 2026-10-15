variable "WATTS_IMAGE_TAG" {
  default = "0.1.0"
}

group "default" {
  targets = ["pi-ralph", "coder-golang"]
}

target "pi-ralph" {
  context = "registry/pi_ralph"
  tags    = ["watts/pi-ralph:${WATTS_IMAGE_TAG}"]
}

target "coder-golang" {
  context = "registry/coder_golang"
  tags    = ["watts/coder-golang:${WATTS_IMAGE_TAG}"]
  contexts = {
    watts-agent-base = "target:pi-ralph"
  }
}
