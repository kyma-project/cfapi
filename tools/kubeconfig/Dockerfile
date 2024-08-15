FROM ruby:3.3

#ENV BTP_CLI_VERSION=2.64.0
#ENV TOOLS_URL=https://tools.hana.ondemand.com
#
#RUN curl --fail --silent --location --cookie eula_3_2_agreed="$TOOLS_URL/developer-license-3_2.txt" \
#  --url "$TOOLS_URL/additional/btp-cli-linux-amd64-$BTP_CLI_VERSION.tar.gz" 

ENV UAA_URL="https://uaa.cf.eu10.hana.ondemand.com"
ENV OIDC_PREFIX="sap.ids"
ENV YQ_VERSION=v4.44.3


RUN gem install cf-uaac
RUN wget https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_amd64.tar.gz -O - | tar xz && mv yq_linux_amd64 /usr/bin/yq
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"  && chmod +x kubectl && mv kubectl /usr/bin/


WORKDIR /genkc
COPY gen-kubeconfig.sh . 
ADD serviceaccount.yaml . 

WORKDIR /work
ENTRYPOINT ["/genkc/gen-kubeconfig.sh"]