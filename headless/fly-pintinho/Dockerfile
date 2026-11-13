# Pintinho no Fly.io: bot + relay + Srmaniaco e 3 convidadas numa máquina só.
# Contexto montado por preparar.sh (cópias, sem cookie). O cookie entra como segredo do Fly
# (COOKIE_PRINCIPAL, COOKIE_NOVAS, COOKIE_ATOS3); os renovados ficam no volume /data.
FROM golang:1.26 AS build
WORKDIR /ws
COPY codigo/ /ws/
RUN cd /ws/bot && CGO_ENABLED=1 go build -o /out/ddbot ./cmd/ddbot \
 && cd /ws/ddrelay && CGO_ENABLED=1 go build -o /out/ddrelay ./cmd/ddrelay

FROM golang:1.26
# ONNX Runtime para o reconhecimento local do bot (onnxruntime_go v1.24.0).
RUN curl -fsSL https://github.com/microsoft/onnxruntime/releases/download/v1.24.4/onnxruntime-linux-x64-1.24.4.tgz \
  | tar -xz -C /opt && ln -s /opt/onnxruntime-linux-x64-1.24.4/lib/libonnxruntime.so* /usr/local/lib/ && ldconfig
ENV DDBOT_ONNXRUNTIME_LIBRARY=/opt/onnxruntime-linux-x64-1.24.4/lib/libonnxruntime.so
# OCR do bot-ddtank (src/verificacao_ocr.py) com o SDK ddddocr.
RUN apt-get update && apt-get install -y --no-install-recommends python3-pip procps && rm -rf /var/lib/apt/lists/* \
 && pip3 install --break-system-packages --no-cache-dir numpy onnxruntime Pillow opencv-python-headless
COPY --from=build /out/ddbot /out/ddrelay /app/
COPY codigo/bot/lua/ /app/lua/
COPY lua-pintinho/ /app/lua/
COPY ddddocr/ /ddddocr/ddddocr/
COPY bot-ddtank-src/ /bot-ddtank/src/
COPY painel/ /app/painel/
COPY secret/ /secret/
ENV CAPTCHA_BANCO=/data/captchas
CMD ["sh", "/secret/iniciar.sh"]
