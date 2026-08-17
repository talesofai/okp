import { Surface, Text } from "@cloudflare/kumo";
import type { DomainActivityPoint } from "../api/client";
import { formatShortDate } from "../lib/time";

const WIDTH = 720;
const HEIGHT = 190;
const PADDING_X = 12;
const PADDING_Y = 18;

function linePath(points: DomainActivityPoint[], value: (point: DomainActivityPoint) => number, max: number): string {
  if (points.length === 0) return "";
  const denominator = Math.max(points.length - 1, 1);
  return points.map((point, index) => {
    const x = PADDING_X + (index / denominator) * (WIDTH - PADDING_X * 2);
    const y = HEIGHT - PADDING_Y - (value(point) / max) * (HEIGHT - PADDING_Y * 2);
    return `${index === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
  }).join(" ");
}

export function ActivityChart({ points }: { points: DomainActivityPoint[] }) {
  const max = Math.max(1, ...points.map((point) => point.activity));
  const activityPath = linePath(points, (point) => point.activity, max);
  const createdPath = linePath(points, (point) => point.created, max);
  const updatedPath = linePath(points, (point) => point.updated, max);
  const totalCreated = points.reduce((sum, point) => sum + point.created, 0);
  const totalUpdated = points.reduce((sum, point) => sum + point.updated, 0);

  return (
    <Surface style={{ padding: "18px 20px", marginBottom: 24 }}>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 16, alignItems: "baseline", flexWrap: "wrap" }}>
        <Text weight="medium">近 30 天活跃度</Text>
        <Text size="xs" color="secondary">
          {points[0]?.date ?? "--"} — {points[points.length - 1]?.date ?? "--"}
        </Text>
      </div>

      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        role="img"
        aria-label="Domain 近 30 天活跃度"
        style={{ display: "block", width: "100%", height: "190px", marginTop: 12, overflow: "visible" }}
      >
        {[0, 0.5, 1].map((ratio) => {
          const y = HEIGHT - PADDING_Y - ratio * (HEIGHT - PADDING_Y * 2);
          return (
            <line
              key={ratio}
              x1={PADDING_X}
              x2={WIDTH - PADDING_X}
              y1={y}
              y2={y}
              stroke="currentColor"
              strokeOpacity={0.12}
              strokeWidth={1}
            />
          );
        })}
        <path
          d={activityPath}
          fill="none"
          stroke="#6366f1"
          strokeOpacity={0.2}
          strokeWidth={18}
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d={activityPath}
          fill="none"
          stroke="#6366f1"
          strokeWidth={2.5}
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d={createdPath}
          fill="none"
          stroke="#10b981"
          strokeWidth={1.5}
          strokeDasharray="4 4"
          strokeLinecap="round"
        />
        <path
          d={updatedPath}
          fill="none"
          stroke="#f59e0b"
          strokeWidth={1.5}
          strokeDasharray="2 4"
          strokeLinecap="round"
        />
        {points.map((point, index) => {
          const denominator = Math.max(points.length - 1, 1);
          const x = PADDING_X + (index / denominator) * (WIDTH - PADDING_X * 2);
          const y = HEIGHT - PADDING_Y - (point.activity / max) * (HEIGHT - PADDING_Y * 2);
          return (
            <circle key={point.date} cx={x} cy={y} r={2.5} fill="#6366f1">
              <title>{`${point.date}: 活跃 ${point.activity}，创建 ${point.created}，更新 ${point.updated}`}</title>
            </circle>
          );
        })}
      </svg>

      <div style={{ display: "flex", gap: 16, flexWrap: "wrap", alignItems: "center", marginTop: 2 }}>
        <Text size="xs" color="secondary">
          <span aria-hidden="true" style={{ display: "inline-block", width: 8, height: 8, marginRight: 5, borderRadius: "50%", background: "#6366f1" }} />
          活跃 {totalCreated + totalUpdated}
        </Text>
        <Text size="xs" color="secondary">
          <span aria-hidden="true" style={{ display: "inline-block", width: 8, height: 8, marginRight: 5, borderRadius: "50%", background: "#10b981" }} />
          创建 {totalCreated}
        </Text>
        <Text size="xs" color="secondary">
          <span aria-hidden="true" style={{ display: "inline-block", width: 8, height: 8, marginRight: 5, borderRadius: "50%", background: "#f59e0b" }} />
          更新 {totalUpdated}
        </Text>
        <Text size="xs" color="secondary" style={{ marginLeft: "auto" }}>
          {formatShortDate(points[0]?.date)} — {formatShortDate(points[points.length - 1]?.date)}
        </Text>
      </div>
    </Surface>
  );
}
