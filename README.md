# Webhook Ranch Hand


# Versions to be Checked
- v2.14.0-alpha12
- v2.13.4-alpha4
- v2.12.8-alpha2

# How to Use

Simply add the alpha or rc version of rancher you want to check rancher/rancher > rancher/webhook go.mods for under `Versions to be Checked`, with a leading `v`. Then run the action. It will create a folder in the repo with the report. Then it will automatically mark the version in the README as `- PROCESSED` and not process it again, unless you manually remove `- PROCESSED` from the README.
