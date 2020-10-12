const config = {
  rtmp: {
    port: 1935,
    chunk_size: 60000,
    gop_cache: true,
    ping: 60,
    ping_timeout: 30,
  },
  http: {
    port: 8888,
    mediaroot: "./broadcasts",
    allow_origin: "*",
  },
  trans: {
    ffmpeg: "/usr/local/bin/ffmpeg",
    tasks: [
      {
        app: "live",
        hls: true,
        hlsFlags: "[hls_time=2:hls_list_size=3:hls_flags=delete_segments]",
        dash: true,
        dashFlags: "[f=dash:window_size=3:extra_window_size=5]",
        mp4: true,
        mp4Flags: "[movflags=faststart]",
      },
    ],
  },
};

// Put this inside your OBS broadcast settings with test as the auth key
// rtmp:127.0.0.1:1935/live

module.exports = config;
