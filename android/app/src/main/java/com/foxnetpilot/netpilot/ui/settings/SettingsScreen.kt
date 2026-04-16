package com.foxnetpilot.netpilot.ui.settings

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel

@Composable
fun SettingsScreen(vm: SettingsViewModel = viewModel()) {
    Column(
        modifier = Modifier.fillMaxSize().padding(16.dp),
    ) {
        Text("设置", style = MaterialTheme.typography.titleLarge)
        Spacer(Modifier.height(16.dp))
        Text(
            "TODO:订阅管理 / 故障自动切换 / 备份导入导出 / API Token",
            style = MaterialTheme.typography.bodyMedium,
        )
    }
}
