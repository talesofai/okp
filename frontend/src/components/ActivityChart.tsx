import { Surface, Text } from "@cloudflare/kumo";
import type { DomainActivityPoint } from "../api/client";
import { formatShortDate } from "../lib/time";

const WIDTH = 760;
const PANEL_HEIGHT = 96;
const PANEL_GAP = 10;
const PANEL_PADDING_X = 14;
const PANEL_PADDING_TOP = 32;
const PANEL_PADDING_BOTTOM = 12;
const CHART_HEIGHT = PANEL_HEIGHT * 3 + PANEL_GAP * 2;

type PlotPoint = { x: number; y: number };

type ActivitySeries = {
  id: string;
  label: string;
  color: string;
  value: (point: DomainActivityPoint) => number;
};

const series: ActivitySeries[] = [
  { id: "activity", label: "活跃", color: "#b9c9c7", value: (point) => point.activity },
  { id: "created", label: "创建", color: "#9fb5b3", value: (point) => point.created },
  { id: "updated", label: "更新", color: "#c8c0ad", value: (point) => point.updated },
];

function smoothPath(points: PlotPoint[]): string {
  if (points.length === 0) return "";
  if (points.length === 1) return `M ${points[0].x.toFixed(2)} ${points[0].y.toFixed(2)}`;

  let path = `M ${points[0].x.toFixed(2)} ${points[0].y.toFixed(2)}`;
  for (let index = 0; index < points.length - 1; index += 1) {
    const previous = points[index - 1] ?? points[index];
    const current = points[index];
    const next = points[index + 1];
    const following = points[index + 2] ?? next;
    const controlOne = {
      x: current.x + (next.x - previous.x) / 6,
      y: current.y + (next.y - previous.y) / 6,
    };
    const controlTwo = {
      x: next.x - (following.x - current.x) / 6,
      y: next.y - (following.y - current.y) / 6,
    };
    path += ` C ${controlOne.x.toFixed(2)} ${controlOne.y.toFixed(2)}, ${controlTwo.x.toFixed(2)} ${controlTwo.y.toFixed(2)}, ${next.x.toFixed(2)} ${next.y.toFixed(2)}`;
  }
  return path;
}

function areaPath(points: PlotPoint[], baseline: number): string {
  if (points.length === 0) return "";
  const curve = smoothPath(points);
  const first = points[0];
  const last = points[points.length - 1];
  return `${curve} L ${last.x.toFixed(2)} ${baseline.toFixed(2)} L ${first.x.toFixed(2)} ${baseline.toFixed(2)} Z`;
}

function plotPoints(
  points: DomainActivityPoint[],
  value: (point: DomainActivityPoint) => number,
  max: number,
  panelIndex: number,
): PlotPoint[] {
  const panelTop = panelIndex * (PANEL_HEIGHT + PANEL_GAP);
  const plotTop = panelTop + PANEL_PADDING_TOP;
  const baseline = panelTop + PANEL_HEIGHT - PANEL_PADDING_BOTTOM;
  const denominator = Math.max(points.length - 1, 1);

  return points.map((point, index) => ({
    x: PANEL_PADDING_X + (index / denominator) * (WIDTH - PANEL_PADDING_X * 2),
    y: baseline - (value(point) / max) * (baseline - plotTop),
  }));
}

function formatTotal(value: number): string {
  return value.toLocaleString("zh-CN");
}

export function ActivityChart({ points }: { points: DomainActivityPoint[] }) {
  const totals = series.map((item) => points.reduce((sum, point) => sum + item.value(point), 0));

  return (
    <Surface
      style={{
        padding: 0,
        marginBottom: 24,
        overflow: "hidden",
        background: "#070909",
        border: "1px solid rgba(157, 174, 173, 0.24)",
        color: "#d8e1df",
      }}
    >
      <div style={{ padding: "18px 20px 14px", display: "flex", justifyContent: "space-between", gap: 16, alignItems: "baseline", flexWrap: "wrap" }}>
        <Text weight="medium" style={{ color: "#e2e9e7" }}>近 30 天活跃度</Text>
        <Text size="xs" style={{ color: "rgba(216, 225, 223, 0.6)", fontVariantNumeric: "tabular-nums" }}>
          {formatShortDate(points[0]?.date)} — {formatShortDate(points[points.length - 1]?.date)}
        </Text>
      </div>

      <svg
        viewBox={`0 0 ${WIDTH} ${CHART_HEIGHT}`}
        preserveAspectRatio="none"
        role="img"
        aria-label="Domain 近 30 天活跃度，包含活跃、创建、更新三组面积图"
        style={{ display: "block", width: "100%", height: `${CHART_HEIGHT}px`, padding: "0 12px", boxSizing: "border-box" }}
      >
        <defs>
          {series.map((item) => (
            <linearGradient key={item.id} id={`okp-${item.id}-area`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={item.color} stopOpacity="0.5" />
              <stop offset="72%" stopColor={item.color} stopOpacity="0.16" />
              <stop offset="100%" stopColor={item.color} stopOpacity="0.02" />
            </linearGradient>
          ))}
        </defs>

        {series.map((item, seriesIndex) => {
          const max = Math.max(1, ...points.map(item.value));
          const panelTop = seriesIndex * (PANEL_HEIGHT + PANEL_GAP);
          const baseline = panelTop + PANEL_HEIGHT - PANEL_PADDING_BOTTOM;
          const plotted = plotPoints(points, item.value, max, seriesIndex);

          return (
            <g key={item.id}>
              <title>{`${item.label}：${formatTotal(totals[seriesIndex])}`}</title>
              <rect
                x="0.5"
                y={panelTop + 0.5}
                width={WIDTH - 1}
                height={PANEL_HEIGHT - 1}
                rx="8"
                fill="#0b0e0f"
                stroke="rgba(166, 181, 180, 0.22)"
              />
              <text
                x={PANEL_PADDING_X + 4}
                y={panelTop + 20}
                fill="rgba(225, 234, 232, 0.72)"
                fontSize="11"
                fontFamily="ui-sans-serif, system-ui, sans-serif"
              >
                {item.label}
              </text>
              <text
                x={WIDTH - PANEL_PADDING_X - 4}
                y={panelTop + 20}
                textAnchor="end"
                fill="rgba(225, 234, 232, 0.72)"
                fontSize="11"
                fontFamily="ui-monospace, SFMono-Regular, Menlo, monospace"
              >
                {formatTotal(totals[seriesIndex])}
              </text>
              <line
                x1={PANEL_PADDING_X}
                x2={WIDTH - PANEL_PADDING_X}
                y1={baseline}
                y2={baseline}
                stroke="rgba(166, 181, 180, 0.14)"
                strokeWidth="1"
              />
              {plotted.length > 0 && (
                <path
                  d={areaPath(plotted, baseline)}
                  fill={`url(#okp-${item.id}-area)`}
                  stroke={item.color}
                  strokeOpacity="0.8"
                  strokeWidth="1.25"
                  strokeLinejoin="round"
                />
              )}
              {points.map((point, index) => {
                const plottedPoint = plotted[index];
                if (!plottedPoint) return null;
                return (
                  <circle key={`${item.id}-${point.date}`} cx={plottedPoint.x} cy={plottedPoint.y} r="8" fill="transparent">
                    <title>{`${point.date}：${item.label} ${item.value(point)}`}</title>
                  </circle>
                );
              })}
            </g>
          );
        })}
      </svg>

      <div style={{ padding: "12px 20px 16px", display: "flex", gap: 16, flexWrap: "wrap", alignItems: "center" }}>
        {series.map((item, index) => (
          <Text key={item.id} size="xs" style={{ color: "rgba(216, 225, 223, 0.68)", fontVariantNumeric: "tabular-nums" }}>
            <span aria-hidden="true" style={{ display: "inline-block", width: 7, height: 7, marginRight: 6, borderRadius: "50%", background: item.color, verticalAlign: "1px" }} />
            {item.label} {formatTotal(totals[index])}
          </Text>
        ))}
        <Text size="xs" style={{ marginLeft: "auto", color: "rgba(216, 225, 223, 0.42)" }}>按日</Text>
      </div>
    </Surface>
  );
}
