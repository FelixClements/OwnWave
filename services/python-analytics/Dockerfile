FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    libsndfile1 \
    gcc \
    libpq-dev \
    libev-dev \
    libffi-dev \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Genre models are optional (ENABLE_GENRE_ANALYSIS defaults off). essentia.upf.edu
# is often unreachable from CI, so skip unless DOWNLOAD_GENRE_MODELS=true.
ARG DOWNLOAD_GENRE_MODELS=false
RUN mkdir -p /app/models && \
    if [ "$DOWNLOAD_GENRE_MODELS" = "true" ]; then \
      curl --connect-timeout 15 --max-time 90 --retry 2 -fL \
        -o /app/models/discogs-effnet-bs64-1.pb \
        https://essentia.upf.edu/models/feature-extractors/discogs-effnet/discogs-effnet-bs64-1.pb && \
      curl --connect-timeout 15 --max-time 90 --retry 2 -fL \
        -o /app/models/genre_discogs400-discogs-effnet-1.pb \
        https://essentia.upf.edu/models/classification-heads/genre_discogs400/genre_discogs400-discogs-effnet-1.pb && \
      curl --connect-timeout 15 --max-time 90 --retry 2 -fL \
        -o /app/models/genre_discogs400-discogs-effnet-1.json \
        https://essentia.upf.edu/models/classification-heads/genre_discogs400/genre_discogs400-discogs-effnet-1.json; \
    fi

COPY src ./src

RUN useradd -u 1000 -m ownwave && chown -R ownwave:ownwave /app

ENV PYTHONPATH=/app/src
ENV PYTHONUNBUFFERED=1
ENV TF_ENABLE_ONEDNN_OPTS=1
ENV TF_CPP_MIN_LOG_LEVEL=2

USER ownwave

EXPOSE 8000

CMD ["uvicorn", "api:app", "--host", "0.0.0.0", "--port", "8000"]
