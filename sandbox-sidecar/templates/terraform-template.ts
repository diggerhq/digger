// templates/terraform-template.ts
import { Template } from "e2b";

export function terraformTemplate(version: string) {
  // version like "1.5.7"
  return Template()
    .fromUbuntuImage("22.04")

    // root for system-level install
    .setUser("root")
    .runCmd("apt-get update && apt-get install -y wget unzip ca-certificates")
    .runCmd(`
      set -e
      cd /tmp
      echo "Installing Terraform ${version}..."
      wget -O terraform.zip https://releases.hashicorp.com/terraform/${version}/terraform_${version}_linux_amd64.zip
      unzip terraform.zip
      mv terraform /usr/local/bin/terraform
      chmod +x /usr/local/bin/terraform
      rm terraform.zip
    `)

    // back to normal user for sandbox runtime
    .setUser("user");
}
