# Webhook Ranch Hand


# Versions to be Checked
- v2.9.12-alpha3
- v2.10.10-alpha3
- v2.11.6-alpha3
- v2.12.2-alpha2
- v2.11.5-alpha3 - PROCESSED
- v2.12.1-alpha4 - PROCESSED
- v2.9.11-alpha3 - PROCESSED
- v2.11.5-alpha2 - PROCESSED
- v2.12.1-alpha3 - PROCESSED
- v2.10.9-alpha2 - PROCESSED
- v2.12.1-alpha2 - PROCESSED
- v2.10.9-alpha1 - PROCESSED
- v2.12.1-alpha1 - PROCESSED
- v2.11.5-alpha1 - PROCESSED
- v2.9.11-alpha2 - PROCESSED

# How to Use

Simply add the alpha or rc version of rancher you want to check rancher/rancher > rancher/webhook go.mods for under `Versions to be Checked`, with a leading `v`. Then run the action. It will create a folder in the repo with the report. Then it will automatically mark the version in the README as `- PROCESSED` and not process it again, unless you manually remove `- PROCESSED` from the README.
