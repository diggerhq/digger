import { Template } from "e2b";


const terraform15 = Template()
  .fromUbuntuImage("22.04")
  .setUser("root")
  .runCmd("apt-get update && apt-get install -y wget unzip")
  .runCmd(`
    cd /tmp && \
    wget -O terraform.zip https://releases.hashicorp.com/terraform/1.5.7/terraform_1.5.7_linux_amd64.zip && \
    unzip terraform.zip && \
    mv terraform /usr/local/bin/terraform && \
    chmod +x /usr/local/bin/terraform && \
    rm terraform.zip
  `)
  .setUser("user")

  
export const template = terraform15;