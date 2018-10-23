package plugins

const awsCred = `
mkdir -p /home/%s/.aws/

cat << EOT > /home/%s/.aws/credentials
%s
EOT`
