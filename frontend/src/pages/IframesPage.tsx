import { useParams } from "react-router-dom";

export default function IframesPage() {
  const { "*": splat } = useParams<{ "*": string }>();
  const url = splat ? decodeURIComponent(splat) : "";

  if (!url) return null;

  const isDark = document.documentElement.classList.contains("dark");
  const hasQuery = url.includes("?");
  const themedUrl = url.includes("theme=")
    ? url
    : `${url}${hasQuery ? "&" : "?"}theme=${isDark ? "dark" : "light"}`;

  return (
    <div className="fixed top-[90px] right-0 bottom-0 left-0 bg-bg">
      <iframe
        src={themedUrl}
        title="External service"
        className="w-full h-full border-0"
        style={{ colorScheme: isDark ? "dark" : "light" }}
        sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
      />
    </div>
  );
}
