import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/constants.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/version.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  Future<Result<AppVersion>>? _versionFuture;

  @override
  void initState() {
    super.initState();
    _checkVersion();
  }

  void _checkVersion() {
    final future = AppScope.of(context).api.update.getReleaseVersion();
    setState(() {
      _versionFuture = future;
    });
  }

  Future<void> _toggleIPv6(bool value) async {
    final api = AppScope.of(context).api;
    await api.session.saveUseIPv6(value);
    api.setUseIPv6(value);
    setState(() {});
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(value ? '已切换到 IPv6 镜像' : '已切换到 IPv4 主机')),
    );
  }

  @override
  Widget build(BuildContext context) {
    final session = AppScope.of(context).api.session;
    const themeNames = ['跟随系统', '浅色', '深色'];
    return Scaffold(
      appBar: AppBar(title: const Text('设置')),
      body: AnimatedBuilder(
        animation: Listenable.merge([session.themeMode, session.fontScale]),
        builder: (context, _) {
          final themeIndex = session.themeMode.value.clamp(0, 2).toInt();
          return ListView(
            children: [
              SwitchListTile(
                title: const Text('使用 IPv6 镜像'),
                subtitle: const Text('bbs6.byr.cn（openIPv6）'),
                value: Constants.useIPv6,
                onChanged: _toggleIPv6,
              ),
              const Divider(),
              ListTile(
                title: const Text('主题'),
                subtitle: Text(themeNames[themeIndex]),
                trailing: DropdownButton<int>(
                  value: themeIndex,
                  items: [
                    for (var i = 0; i < themeNames.length; i++)
                      DropdownMenuItem(value: i, child: Text(themeNames[i])),
                  ],
                  onChanged: (value) {
                    if (value == null) return;
                    session.saveThemeMode(value);
                  },
                ),
              ),
              ListTile(
                title: const Text('字体大小'),
                subtitle: Slider(
                  value: session.fontScale.value,
                  min: 0.8,
                  max: 1.4,
                  divisions: 6,
                  label: '${(session.fontScale.value * 100).round()}%',
                  onChanged: (value) => session.fontScale.value = value,
                  onChangeEnd: session.saveFontScale,
                ),
              ),
              const Divider(),
              ListTile(
                title: const Text('检查更新'),
                subtitle: FutureBuilder<Result<AppVersion>>(
                  future: _versionFuture,
                  builder: (context, snapshot) {
                    if (snapshot.connectionState != ConnectionState.done) {
                      return const Text('检查中…');
                    }
                    final result = snapshot.data;
                    if (result == null || result.isFailure) {
                      return Text(result?.message ?? '检查失败');
                    }
                    final versions = result.data?.versions ?? const <Version>[];
                    if (versions.isEmpty) return const Text('没有可用版本');
                    final latest = versions.first;
                    return Text(
                      '最新版本：${latest.version ?? '-'}\n'
                      '${latest.changelog ?? ''}',
                    );
                  },
                ),
              ),
              ListTile(
                leading: const Icon(Icons.info_outline),
                title: const Text('关于'),
                subtitle: const Text('BYR Client\n北邮人论坛跨端客户端'),
              ),
            ],
          );
        },
      ),
    );
  }
}
