# OwnWave
Open-Source AI Smart Radio Clone

An intelligent, self-hosted streaming application designed to transform a local archive of lossless (FLAC) music into a personalized, continuous radio broadcasting network. 

Unlike standard media servers that rely on manual playlists or basic shuffle queues, this system uses advanced audio analysis and machine learning to analyze the structural properties of your files. It treats your hard drive as a private radio station engine—dynamically grouping music by tempo, acoustic texture, emotional valence, and micro-genres to curate seamless, infinite radio channels.

## Architectural Concept & Division of Labor

To manage massive libraries consisting of hundreds of gigabytes of high-resolution audio without introducing latency, the system splits operations across three specialized environments:

*   **t3-web-frontend (User Interface Layer)**: Handshakes with the API, sends media control actions, and directly receives the audio streams.
*   **go-api-server (Traffic & Delivery Layer)**: Pulls data from the database, reads player state, and pipes transcoded or raw audio buffers down to the user interface.
*   **python-analytics-engine (The Cognitive Brain)**: Works purely in the background to analyze files, extract metadata, calculate audio vectors (BPM, mood), and write structured genre clusters back into the database.
*   **Shared Database (The Unified State)**: Acts as the central data bridge holding all normalized song files, playlists, configuration parameters, and AI-generated clustering vectors.

## 📂 Repository Structure

The project uses a monorepo structure. This layout isolates the foundational web application and backend microservices from day one, while designating explicit entry points for native ecosystem deployments when the project expands.

```text
├── apps/
│   ├── t3-web-frontend/         # Current Web Application (Next.js, Tailwind, tRPC)
│   │
│   # FUTURE NATIVE EXPANSION CLIENTS (Planned Modules)
│   ├── native-android/          # Future Native Android build space
│   ├── native-macos/            # Future macOS Desktop wrapper or Swift interface
│   └── native-windows/          # Future C#/.NET or native Windows client space
│
├── services/
│   ├── go-api-server/           # High-performance I/O and on-the-fly streaming pipeline
│   └── python-analytics/        # File scanner, metadata parser, and digital signal processing
│
└── docker-compose.yml           # Production & development infrastructure configuration
```

## 🚀 Detailed Component Descriptions

### 1. Web Frontend (apps/t3-web-frontend)
The presentation layer is responsible for translating abstract music streams into a responsive, high-fidelity visual experience heavily inspired by modern premium streaming clients.

*   **Type-Safe Communications**: By utilizing tRPC as its data backbone, the frontend communicates with internal backend components with complete type safety. Any modification to a track model or station payload instantly bubbles up to the interface layer, eliminating runtime API data bugs.
*   **Liquid Design & Media Controls**: Built using Tailwind CSS, the UI provides an adaptive dark-mode layout optimized for continuous playback. It includes fluid sidebar navigation, deep search capabilities, real-time audio visualizers, and responsive media key integrations.
*   **Smart Audio Buffering**: Built on top of the native HTML5 Audio API, the player manages complex playback states, gapless track transitions, and dynamic buffering rules to handle large stream payloads smoothly.

### 2. Traffic & Delivery Layer (services/go-api-server)
Written in Go, this component operates as a high-performance network router and audio pipeline. It is intentionally decoupled from data-heavy calculation tasks to ensure that serving audio files requires minimal CPU and RAM overhead.

*   **On-The-Fly Audio Transcoding**: While desktop browsers handle raw FLAC playback effortlessly, mobile and Safari-based browsers frequently fail or stutter when streaming lossless formats. Go intercepts incoming user requests and dynamically assesses the client browser profile. If the device lacks native optimization for high-bitrate FLAC, Go streams the file through an in-memory FFmpeg engine, instantly squeezing it into a highly compatible 320kbps MP3 stream on the fly.
*   **Low-Latency Indexing**: Go's concurrency model handles massive track maps effortlessly. It fields incoming queries from the T3 frontend, pulls pre-computed radio queues from the database in microseconds, opens the local storage path, and streams raw audio data without blocking system processes.

### 3. The Cognitive Engine (services/python-analytics)
The data-crunching back-end worker. Running completely asynchronously from the rest of the application, Python acts as a background pipeline that handles deep directory indexing, statistical evaluation, and automated curation.

*   **Lossless Digital Signal Processing (DSP)**: Python utilizes processing libraries like librosa and essentia to extract deep sonic data directly from the audio waveforms of your FLAC files. It calculates specific mathematical audio attributes such as rhythmic tempo (BPM), acoustic energy levels, danceability metrics, and valence (the emotional mood or brightness of a song).
*   **Metadata Normalization**: It scans massive folder structures to read ID3/Vorbis tags, extracting artist credits, album information, release years, and original genre notes. It normalizes this data to fix inconsistent tags.
*   **AI-Driven Radio Matrixing**: Instead of relying on static, unchanging playlists, Python processes these extracted audio attributes using unsupervised clustering algorithms (such as K-Means or Vector Embeddings). It continuously categorizes your library into interconnected "station profiles." This lets the app generate randomized, highly cohesive radio streams where every consecutive track perfectly matches the vibe, mood, and style of the selected channel.
