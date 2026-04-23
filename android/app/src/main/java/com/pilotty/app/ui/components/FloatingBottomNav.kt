package com.pilotty.app.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
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
 * 底部导航 —— 设计稿 .bottomnav: flush 风格, 无 floating pill。
 *   - 56dp 高, 5 等分 grid
 *   - 顶部 1px hairline 作为与内容的分界
 *   - active tab: top 顶部 2dp x 22dp 柠檬绿短横 (::after)
 *   - label 下 10sp 小字
 *
 * 不做 backdrop blur / 悬浮阴影, 这是 Mission Control 设计语言的刻意选择
 * (节制的 chrome, content-first)。
 */
@Composable
fun FloatingBottomNav(
    items: List<NavItem>,
    current: String,
    onTab: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val pc = LocalPilottyColors.current
    Column(
        modifier = modifier
            .fillMaxWidth()
            .background(pc.navBg),
    ) {
        // 顶部 hairline —— 设计稿 shadow-nav: 0 -1px 0 divider
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(1.dp)
                .background(pc.hairline),
        )
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .height(56.dp),
        ) {
            items.forEach { item ->
                NavItemCell(
                    item = item,
                    active = current == item.route,
                    onClick = { if (current != item.route) onTab(item.route) },
                    modifier = Modifier.weight(1f).fillMaxHeight(),
                )
            }
        }
    }
}

@Composable
private fun NavItemCell(
    item: NavItem,
    active: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val pc = LocalPilottyColors.current
    val fg = if (active) pc.ink else pc.ink2
    Box(
        modifier = modifier.clickable(onClick = onClick),
    ) {
        // 顶部 accent 短横 (active 专属)
        if (active) {
            Box(
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .padding(top = 4.dp)
                    .size(width = 22.dp, height = 2.dp)
                    .clip(RoundedCornerShape(1.dp))
                    .background(pc.accent),
            )
        }
        Column(
            modifier = Modifier.align(Alignment.Center),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(3.dp),
        ) {
            Icon(
                imageVector = item.icon,
                contentDescription = item.label,
                tint = fg,
                modifier = Modifier.size(22.dp),
            )
            Text(
                text = item.label,
                color = fg,
                fontSize = 10.sp,
                fontWeight = FontWeight.Medium,
                letterSpacing = 0.2.sp,
            )
        }
    }
}
