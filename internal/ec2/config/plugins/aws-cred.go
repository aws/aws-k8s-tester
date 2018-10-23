package plugins

const aws_cred = `
mkdir -p /home/%s/.aws/

cat << EOT > /home/%s/.aws/credentials
%s
EOT`
