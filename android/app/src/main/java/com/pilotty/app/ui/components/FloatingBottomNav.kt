package com.pilotty.app.ui.components

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.theme.LocalPilottyColors

data class NavItem(
    val route: String,
    val label: String,
    val icon: ImageVector,
)

/**
 * Floating pill 底部导航。对齐设计稿:
 *   - 检测底部 30dp 内 inset, 左右 16dp margin
 *   - 半透明胶囊 + backdrop blur (Android 31+ 真 blur;低版本退化为不透明)
 *   - active tab 底部 2dp 柠檬绿短横
 *
 * 注: Compose 目前没有 API-level 原生 backdrop blur;我们用半透明 + 阴影
 * 做 visual proxy。真 blur 留给后续 `Modifier.blur(backdrop=true)` 或
 * HazeModifier 第三方库。
 */
@Composable
fun FloatingBottomNav(
    items: List<NavItem>,
    current: String,
    onTab: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val pc = LocalPilottyColors.current
    Box(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 0.dp)
            .padding(bottom = 24.dp),
        contentAlignment = Alignment.Center,
    ) {
        Surface(
            modifier = Modifier
                .widthIn(max = 340.dp)
                .fillMaxWidth()
                .shadow(
                    elevation = 20.dp,
                    shape = RoundedCornerShape(999.dp),
                    clip = false,
                ),
            shape = RoundedCornerShape(999.dp),
            color = pc.navBg,
            border = if (pc.isDark) BorderStroke(1.dp, pc.hairlineStrong) else BorderStroke(0.5.dp, pc.hairline),
        ) {
            Row(
                modifier = Modifier
                    .padding(horizontal = 10.dp, vertical = 8.dp)
                    .fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                items.forEach { item ->
                    NavItemPill(
                        item = item,
                        active = current == item.route,
                        onClick = { if (current != item.route) onTab(item.route) },
                        modifier = Modifier.weight(1f),
                    )
                }
            }
        }
    }
}

@Composable
private fun NavItemPill(
    item: NavItem,
    active: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val pc = LocalPilottyColors.current
    val fg = if (active) pc.ink else pc.ink4
    Box(
        modifier = modifier
            .clip(RoundedCornerShape(999.dp))
            .clickable(onClick = onClick)
            .padding(vertical = 6.dp),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Icon(
                imageVector = item.icon,
                contentDescription = item.label,
                tint = fg,
                modifier = Modifier.size(19.dp),
            )
            Text(
                text = item.label,
                color = fg,
                fontSize = 10.sp,
                fontWeight = FontWeight.Medium,
                letterSpacing = 0.2.sp,
            )
            // active underline
            if (active) {
                Box(
                    modifier = Modifier
                        .padding(top = 1.dp)
                        .size(width = 18.dp, height = 2.dp)
                        .clip(RoundedCornerShape(1.dp))
                        .background(pc.accent),
                )
            } else {
                // 占位, 保证高度一致
                Spacer(Modifier.size(width = 18.dp, height = 2.dp).padding(top = 1.dp))
            }
        }
    }
}
