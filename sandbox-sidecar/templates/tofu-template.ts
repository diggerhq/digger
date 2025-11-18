// templates/tofu-template.ts
import { Template } from "e2b";

export function tofuTemplate(version: string) {
  return Template()
    .fromUbuntuImage("22.04")
    .setUser("root")
    .runCmd("apt-get update && apt-get install -y wget ca-certificates")
    .runCmd(`
      set -e
      cd /tmp
      echo "Installing OpenTofu ${version}..."
      wget -O tofu.tar.gz https://github.com/opentofu/opentofu/releases/download/v${version}/tofu_${version}_linux_amd64.tar.gz
      tar -xzf tofu.tar.gz
      mv tofu /usr/local/bin/tofu
      chmod +x /usr/local/bin/tofu
      rm tofu.tar.gz
    `)
    .setUser("user");
}
