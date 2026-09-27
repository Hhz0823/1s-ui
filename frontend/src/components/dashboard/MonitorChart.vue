<template>
  <div class="monitor-chart">
    <Line :data="chartData" :options="<any>chartOptions" />
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { useTheme } from 'vuetify'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Filler,
  Tooltip,
} from 'chart.js'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Filler, Tooltip)

export interface MonitorSeries {
  label: string
  data: number[]
  color: string
}

const props = defineProps<{
  labels: string[]
  series: MonitorSeries[]
  format: (value: number) => string
  max?: number
  // Keeps an idle chart from scaling to fractions (e.g. 1 KB/s for rates).
  suggestedMax?: number
}>()

const theme = useTheme()

const rgba = (hex: string, alpha: number) => {
  const value = String(hex || '').replace('#', '')
  if (!/^[0-9a-f]{6}$/i.test(value)) return `rgba(128, 128, 128, ${alpha})`
  const [red, green, blue] = [0, 2, 4].map(start => Number.parseInt(value.slice(start, start + 2), 16))
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`
}

// chart.js patches the arrays it is given; plain copies keep it away from
// Vue's reactive proxies.
const chartData = computed(() => ({
  labels: [...props.labels],
  datasets: props.series.map(series => ({
    label: series.label,
    data: [...series.data],
    borderColor: series.color,
    backgroundColor: rgba(series.color, 0.12),
    fill: true,
    tension: 0.35,
    borderWidth: 2,
    pointRadius: 0,
    pointHoverRadius: 3,
  })),
}))

const chartOptions = computed(() => {
  const ink = String(theme.current.value.colors['on-surface'] || '#333333')
  return {
    animation: false,
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' },
    plugins: {
      legend: { display: false },
      tooltip: {
        callbacks: {
          label: (context: any) => `${context.dataset.label}: ${props.format(Number(context.parsed.y) || 0)}`,
        },
      },
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { color: rgba(ink, 0.5), maxTicksLimit: 6, maxRotation: 0, font: { size: 11 } },
      },
      y: {
        min: 0,
        max: props.max,
        suggestedMax: props.suggestedMax,
        grid: { color: rgba(ink, 0.07) },
        border: { display: false },
        ticks: {
          color: rgba(ink, 0.5),
          maxTicksLimit: 5,
          font: { size: 11 },
          callback: (value: any) => props.format(Number(value) || 0),
        },
      },
    },
  }
})
</script>

<style scoped>
.monitor-chart {
  position: relative;
  width: 100%;
  height: 260px;
}

@media (max-width: 600px) {
  .monitor-chart {
    height: 200px;
  }
}
</style>
