interface CoverProps {
  track: string;
  sha: string;
}

export function Cover({ track, sha }: CoverProps) {
  return (
    <div className="cover-box">
      <img src={`/api/tracks/${encodeURIComponent(track)}/cover?v=${sha.slice(0, 16)}`} alt="Cover" loading="lazy" />
    </div>
  );
}
