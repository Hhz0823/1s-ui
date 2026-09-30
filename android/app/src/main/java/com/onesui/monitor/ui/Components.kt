package com.onesui.monitor.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.Format

@Composable
fun StatusDot(online: Boolean, size: Dp = 10.dp) {
    Box(
        Modifier
            .size(size)
            .clip(CircleShape)
            .background(if (online) StatusColors.online else StatusColors.offline)
    )
}

/** A labelled thin bar, the building block of the Komari-style server card. */
@Composable
fun UsageBar(label: String, percent: Double, detail: String, modifier: Modifier = Modifier) {
    val value = percent.coerceIn(0.0, 100.0)
    Column(modifier) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text(label, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.width(6.dp))
            Text(
                detail,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
            )
            Text(Format.percent(value), style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.SemiBold)
        }
        Spacer(Modifier.height(4.dp))
        Box(
            Modifier
                .fillMaxWidth()
                .height(6.dp)
                .clip(RoundedCornerShape(3.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant)
        ) {
            Box(
                Modifier
                    .fillMaxWidth((value / 100.0).toFloat())
                    .height(6.dp)
                    .clip(RoundedCornerShape(3.dp))
                    .background(usageColor(value))
            )
        }
    }
}

@Composable
fun LabelValue(label: String, value: String, modifier: Modifier = Modifier) {
    Column(modifier.padding(vertical = 4.dp)) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value.ifBlank { "-" }, style = MaterialTheme.typography.bodyMedium, maxLines = 2, overflow = TextOverflow.Ellipsis)
    }
}

data class ChartSeries(val name: String, val color: Color, val values: List<Double>)

/**
 * Minimal line chart: shared X over the samples, filled area under each line.
 * [maxValue] fixes the scale (percent charts); null scales to the data.
 */
@Composable
fun LineChart(
    series: List<ChartSeries>,
    maxValue: Double?,
    formatValue: (Double) -> String,
    startLabel: String,
    endLabel: String,
    modifier: Modifier = Modifier,
) {
    val grid = MaterialTheme.colorScheme.outlineVariant
    val dataMax = series.flatMap { it.values }.maxOrNull() ?: 0.0
    val top = maxValue ?: dataMax.coerceAtLeast(1.0) * 1.15
    Column(modifier) {
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            series.forEach { s ->
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Box(Modifier.size(8.dp).clip(CircleShape).background(s.color))
                    Spacer(Modifier.width(4.dp))
                    val last = s.values.lastOrNull()
                    Text(
                        if (last != null) "${s.name} ${formatValue(last)}" else s.name,
                        style = MaterialTheme.typography.labelSmall,
                    )
                }
            }
            Spacer(Modifier.weight(1f))
            Text("峰值 ${formatValue(dataMax)}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Spacer(Modifier.height(6.dp))
        Canvas(
            Modifier
                .fillMaxWidth()
                .height(120.dp)
        ) {
            for (i in 0..4) {
                val y = size.height * i / 4f
                drawLine(grid, Offset(0f, y), Offset(size.width, y), strokeWidth = 1f)
            }
            series.forEach { s ->
                if (s.values.size < 2) return@forEach
                val stepX = size.width / (s.values.size - 1)
                val line = Path()
                s.values.forEachIndexed { index, v ->
                    val x = index * stepX
                    val y = size.height - (v / top).toFloat().coerceIn(0f, 1f) * size.height
                    if (index == 0) line.moveTo(x, y) else line.lineTo(x, y)
                }
                val area = Path().apply {
                    addPath(line)
                    lineTo(size.width, size.height)
                    lineTo(0f, size.height)
                    close()
                }
                drawPath(area, Brush.verticalGradient(listOf(s.color.copy(alpha = 0.28f), s.color.copy(alpha = 0.02f))))
                drawPath(line, s.color, style = Stroke(width = 2.dp.toPx()))
            }
        }
        Row(Modifier.fillMaxWidth()) {
            Text(startLabel, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.weight(1f))
            Text(endLabel, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}
