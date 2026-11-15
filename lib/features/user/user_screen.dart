import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/features/blacklist/blacklist_screen.dart';
import 'package:byr_client/features/board/all_boards_screen.dart';
import 'package:byr_client/features/favorite/favorite_screen.dart';
import 'package:byr_client/features/user/history_screen.dart';
import 'package:byr_client/features/login/login_screen.dart';
import 'package:byr_client/features/mail/chat_screen.dart';
import 'package:byr_client/features/mail/mail_screen.dart';
import 'package:byr_client/features/refer/refer_screen.dart';
import 'package:byr_client/features/settings/settings_screen.dart';

class UserPage extends StatefulWidget {
  const UserPage({super.key});

  @override
  State<UserPage> createState() => _UserPageState();
}

class _UserPageState extends State<UserPage> {
  Future<Result<User>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.user.getSelfUserInfo();
    setState(() {
      _future = future;
    });
  }

  Future<void> _logout() async {
    final api = AppScope.of(context).api;
    await api.session.clear();
    api.client.token = null;
    if (!mounted) return;
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (_) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('我'),
        actions: [
          IconButton(
            onPressed: _load,
            icon: const Icon(Icons.refresh),
            tooltip: '刷新',
          ),
        ],
      ),
      body: FutureBuilder<Result<User>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<User>(
            result: snapshot.data,
            onRetry: _load,
            builder: (user) {
              return ListView(
                children: [
                  const SizedBox(height: 16),
                  Center(
                    child: CircleAvatar(
                      radius: 40,
                      backgroundImage: absoluteUrl(user.faceUrl) == null
                          ? null
                          : NetworkImage(absoluteUrl(user.faceUrl)!),
                      child: user.faceUrl == null || user.faceUrl!.isEmpty
                          ? const Icon(Icons.person, size: 40)
                          : null,
                    ),
                  ),
                  const SizedBox(height: 12),
                  Center(
                    child: Text(
                      displayUserName(user),
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                  ),
                  if (user.level != null)
                    Center(
                      child: Text(
                        '等级 ${user.level}',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ),
                  const SizedBox(height: 16),
                  _InfoTile(label: '发帖数', value: '${user.postCount ?? 0}'),
                  _InfoTile(label: '积分', value: '${user.score ?? 0}'),
                  _InfoTile(label: '关注', value: '${user.followNum ?? 0}'),
                  _InfoTile(label: '粉丝', value: '${user.fansNum ?? 0}'),
                  _InfoTile(
                    label: '最后登录',
                    value: formatTimestamp(
                      user.lastLoginTime,
                      alwaysShowYear: true,
                    ),
                  ),
                  const Divider(),
                  ListTile(
                    leading: const Icon(Icons.mail_outline),
                    title: const Text('信箱'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute(builder: (_) => const MailScreen()),
                    ),
                  ),
                  ListTile(
                    leading: const Icon(Icons.chat_bubble_outline),
                    title: const Text('私信'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute(builder: (_) => const ChatListScreen()),
                    ),
                  ),
                  ListTile(
                    leading: const Icon(Icons.star_border),
                    title: const Text('收藏夹'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute(builder: (_) => const FavoriteScreen()),
                    ),
                  ),
                  ListTile(
                    leading: const Icon(Icons.forum_outlined),
                    title: const Text('全部版面'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (_) => const AllBoardsScreen(),
                      ),
                    ),
                  ),
                  ListTile(
                    leading: const Icon(Icons.history),
                    title: const Text('浏览历史'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute(builder: (_) => const HistoryScreen()),
                    ),
                  ),
                  ListTile(
                    leading: const Icon(Icons.notifications_none),
                    title: const Text('提醒'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute(builder: (_) => const ReferScreen()),
                    ),
                  ),
                  ListTile(
                    leading: const Icon(Icons.block),
                    title: const Text('黑名单'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (_) => const BlackListScreen(),
                      ),
                    ),
                  ),
                  ListTile(
                    leading: const Icon(Icons.settings),
                    title: const Text('设置'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute(builder: (_) => const SettingsScreen()),
                    ),
                  ),
                  const Divider(),
                  ListTile(
                    leading: const Icon(Icons.logout),
                    title: const Text('退出登录'),
                    onTap: _logout,
                  ),
                ],
              );
            },
          );
        },
      ),
    );
  }
}

class _InfoTile extends StatelessWidget {
  const _InfoTile({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return ListTile(dense: true, title: Text(label), trailing: Text(value));
  }
}
