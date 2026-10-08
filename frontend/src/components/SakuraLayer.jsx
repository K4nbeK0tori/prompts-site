import React from 'react'

// 12 片花瓣，用负延迟错开，避免整层同时从顶部落下。
const PETALS = Array.from({ length: 12 }, (_, i) => ({
  left: (i * 8.3 + 3) % 96,
  delay: -(i * 1.9),
  duration: 12 + (i % 5) * 2.1,
  scale: (0.66 + (i % 4) * 0.18).toFixed(2),
}))

export default function SakuraLayer({ enabled }) {
  if (!enabled) return null
  return (
    <div className="sakura-layer" aria-hidden="true">
      {PETALS.map((p, i) => (
        <span
          key={i}
          className="petal"
          style={{
            left: `${p.left}vw`,
            animationDelay: `${p.delay}s`,
            animationDuration: `${p.duration}s`,
            '--s': p.scale,
          }}
        />
      ))}
    </div>
  )
}
