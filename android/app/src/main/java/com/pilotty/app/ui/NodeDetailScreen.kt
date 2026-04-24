package com.pilotty.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.pilotty.app.data.NodeDto
import com.pilotty.app.ui.components.*
import com.pilotty.app.ui.qr.NodeQrDialog
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * NodeDetailScreen —— 单节点完整面板。
 * Hero (ring + name + proto + host + 连接 CTA) + Metrics 2x2 + Load + Config rows.
 * 设计稿 06 屏, 整本给 Dark 变体做示范。
 */
@Composable
fun NodeDetailScreen(
    nodeId: String,
    onBack: () -> Unit,
    vm: NodesViewModel = viewModel(),
) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    val node = remember(ui.nodes, nodeId) {
        ui.nodes.firstOrNull { it.tag == nodeId } ?: NodeDto(
            tag = nodeId,
            server = "-",
            port = 0,
            type = "unknown",
            latency = 0,
            alive = false,
            active = false,
        )
    }
    val lat = if (node.latency > 0) node.latency else 0
    var showQr by remember { mutableStateOf(false) }

    if (showQr) {
        NodeQrDialog(nodeTag = node.tag, onDismiss = { showQr = false })
    }

    Column(modifier = Modifier.fillMaxSize().background(pc.bg)) {
        // Top bar
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(
                modifier = Modifier.clickable { onBack() },
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Text("‹", color = pc.ink2, fontSize = 18.sp, fontWeight = FontWeight.Bold)
                Text("Nodes", color = pc.ink2, fontSize = 13.sp)
            }
        }
        HorizontalDivider(color = pc.hairline, thickness = 1.dp)

        Column(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // Hero card
            PilottyCard(modifier = Modifier.fillMaxWidth()) {
                Column(Modifier.padding(20.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Kicker(androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_kicker))
                        Text(
                            if (node.alive)
                                androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_online)
                            else
                                androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_offline),
                            color = pc.ink2,
                            fontSize = 10.5.sp,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = 1.47.sp,
                        )
                    }
                    Spacer(Modifier.height(14.dp))
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                        LatencyRing(ms = lat, size = 72)
                        Column(modifier = Modifier.weight(1f)) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                FlagChip(code = node.tag.take(2).uppercase())
                                Text(
                                    node.tag,
                                    color = pc.ink,
                                    fontSize = 19.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    letterSpacing = (-0.28).sp,
                                )
                            }
                            Spacer(Modifier.height(4.dp))
                            Text(
                                "${node.server}:${node.port}",
                                color = pc.ink3,
                                fontSize = 11.5.sp,
                                fontFamily = FontFamily.Monospace,
                            )
                            Spacer(Modifier.height(6.dp))
                            ProtoBadge(name = node.type)
                        }
                    }
                    Spacer(Modifier.height(16.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        PilottyButton(
                            text = if (node.active) androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_connected) else androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_connect_this),
                            onClick = {
                                if (!node.active) {
                                    vm.switchTo(node.tag)
                                    onBack()
                                }
                            },
                            variant = PilottyButtonVariant.Power,
                            enabled = !node.active,
                            modifier = Modifier.weight(1f),
                        )
                        PilottyButton(
                            text = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_share_qr),
                            onClick = { showQr = true },
                            variant = PilottyButtonVariant.Mono,
                        )
                    }
                }
            }

            SectionHead(
                text = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_metrics_title),
                modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp),
            )
            // 2x2 metrics grid — @VisualOnly: 后端目前不采样 histogram/jitter/loss,
            // 这 4 个 tile 基于当前 latency 估算, 仅保留视觉节奏, 非真实 telemetry
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                MetricTile("Latency P50", "${(lat - 1).coerceAtLeast(0)}", "ms", modifier = Modifier.weight(1f))
                MetricTile("Latency P95", "${lat + 6}", "ms", modifier = Modifier.weight(1f))
            }
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                MetricTile("Jitter", "2.1", "ms", modifier = Modifier.weight(1f))
                MetricTile("Loss", "0.0", "%", modifier = Modifier.weight(1f))
            }
            Text(
                androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_metrics_disclaimer),
                color = pc.ink4,
                fontSize = 10.5.sp,
                fontFamily = FontFamily.Monospace,
                lineHeight = 14.sp,
                modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp),
            )

            // Load card
            PilottyCard(modifier = Modifier.fillMaxWidth()) {
                Column(Modifier.padding(20.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Kicker("Load")
                        val loadPct = (lat / 4).coerceIn(0, 100)
                        Text(
                            "$loadPct%",
                            color = pc.ink,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.SemiBold,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                    Spacer(Modifier.height(10.dp))
                    Progress(pct = (lat / 4).coerceIn(0, 100).toFloat(), height = 8)
                    Spacer(Modifier.height(10.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Text("0", color = pc.ink3, fontSize = 11.sp)
                        Text("50", color = pc.ink3, fontSize = 11.sp)
                        Text("100", color = pc.ink3, fontSize = 11.sp)
                    }
                }
            }

            SectionHead(
                text = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_config),
                modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp),
            )
            CardGroup(modifier = Modifier.fillMaxWidth()) {
                CardRow(
                    label = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_protocol),
                    value = node.type.uppercase(),
                )
                RowDivider()
                CardRow(
                    label = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_endpoint),
                    value = "${node.server}:${node.port}",
                )
                RowDivider()
                CardRow(
                    label = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_status),
                    value = if (node.alive)
                        androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_online_label)
                    else
                        androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_offline_label),
                    valueMono = false,
                )
                RowDivider()
                CardRow(
                    label = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_active),
                    value = if (node.active)
                        androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_yes)
                    else
                        androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.node_detail_no),
                    valueMono = false,
                )
            }

            Spacer(Modifier.height(16.dp))
        }
    }
}

@Composable
private fun MetricTile(
    label: String,
    value: String,
    unit: String,
    modifier: Modifier = Modifier,
) {
    val pc = LocalPilottyColors.current
    PilottyCard(modifier = modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp)) {
            Kicker(label)
            Spacer(Modifier.height(6.dp))
            Row(verticalAlignment = Alignment.Bottom) {
                Text(
                    value,
                    color = pc.ink,
                    fontSize = 22.sp,
                    fontWeight = FontWeight.Bold,
                    letterSpacing = (-0.44).sp,
                    fontFamily = FontFamily.Monospace,
                )
                Spacer(Modifier.width(4.dp))
                Text(unit, color = pc.ink3, fontSize = 11.sp, fontWeight = FontWeight.Medium)
            }
        }
    }
}
