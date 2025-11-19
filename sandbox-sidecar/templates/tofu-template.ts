// templates/tofu-template.ts
import { Template } from "e2b";

export function tofuTemplate(version: string) {
  return Template()
    .fromUbuntuImage("22.04")
    .setUser("root")
    .runCmd("apt-get update && apt-get install -y wget unzip ca-certificates")
    .runCmd(`
      set -e
      cd /tmp
      echo "Installing OpenTofu ${version}..."
      # OpenTofu releases use a zip file, not tar.gz
      wget -O tofu.zip https://github.com/opentofu/opentofu/releases/download/v${version}/tofu_${version}_linux_amd64.zip
      unzip tofu.zip
      chmod +x tofu
      mv tofu /usr/local/bin/tofu
      rm tofu.zip
      # Verify installation
      /usr/local/bin/tofu version
    `)
    .setUser("user");
}
