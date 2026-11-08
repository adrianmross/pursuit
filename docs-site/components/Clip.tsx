// A short looping clip: capped width, its height reserved up front so the page
// doesn't jump as it loads, and a poster until the first frame arrives.
// `src` is the path without an extension; the .webm, .mp4 and .png sit beside each other.
export function Clip({ src, width, height, caption }: { src: string; width: number; height: number; caption: string }) {
  return (
    <figure style={{ maxWidth: Math.min(width, 720), margin: "1.5rem 0" }}>
      <video
        autoPlay
        loop
        muted
        playsInline
        preload="auto"
        poster={`${src}.png`}
        width={width}
        height={height}
        aria-label={caption}
        style={{ width: "100%", height: "auto", aspectRatio: `${width} / ${height}`, borderRadius: 8, border: "1px solid var(--line, rgba(128,128,128,.25))" }}
      >
        <source src={`${src}.webm`} type="video/webm" />
        <source src={`${src}.mp4`} type="video/mp4" />
      </video>
      <figcaption style={{ marginTop: 8, fontSize: 13, opacity: 0.7 }}>{caption}</figcaption>
    </figure>
  )
}

// Marks a feature that needs a plan on the hosted service.
export function Premium() {
  return <span className="badge-premium">Premium</span>
}
