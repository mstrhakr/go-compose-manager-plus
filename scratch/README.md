# Local Uploads

Place the existing VM manager script here, for example as `manager.sh`.
Everything in this directory except this README is ignored by Git. The Docker
build's allowlist excludes this directory too.

Redact passwords, tokens, private keys, registry credentials, hostnames and other
sensitive values before upload. Do not execute uploaded scripts during research.
Treat them as untrusted input; inspect them without logging embedded secrets.

The Compose Manager plugin remains the primary product case study. Script review
will inform actual VM workflows, not dictate the standalone storage or auth model.