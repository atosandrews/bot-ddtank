FROM golang:1.26
RUN apt-get update && apt-get install -y --no-install-recommends protobuf-compiler && rm -rf /var/lib/apt/lists/*
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 \
 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.0
# ONNX Runtime para o reconhecimento local do bot (onnxruntime_go v1.24.0).
RUN curl -fsSL https://github.com/microsoft/onnxruntime/releases/download/v1.24.4/onnxruntime-linux-x64-1.24.4.tgz \
  | tar -xz -C /opt && ln -s /opt/onnxruntime-linux-x64-1.24.4/lib/libonnxruntime.so* /usr/local/lib/ && ldconfig
ENV DDBOT_ONNXRUNTIME_LIBRARY=/opt/onnxruntime-linux-x64-1.24.4/lib/libonnxruntime.so
WORKDIR /ws
# OCR do bot-ddtank (src/verificacao_ocr.py) com o SDK local ddddocr montado em /ddddocr.
RUN apt-get update && apt-get install -y --no-install-recommends python3-pip && rm -rf /var/lib/apt/lists/* \
 && pip3 install --break-system-packages --no-cache-dir numpy onnxruntime Pillow opencv-python-headless
