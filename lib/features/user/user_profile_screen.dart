import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/features/mail/mail_compose_screen.dart';

class UserProfileScreen extends StatefulWidget {
  const UserProfileScreen({super.key, required this.userId});

  final String userId;

  @override
  State<UserProfileScreen> createState() => _UserProfileScreenState();
}

class _UserProfileScreenState extends State<UserProfileScreen> {
  Future<Result<User>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.user.getUserInfo(widget.userId);
    setState(() {
      _future = future;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('用户信息'),
        actions: [
          IconButton(
            tooltip: '发私信',
            onPressed: widget.userId.isEmpty
                ? null
                : () => Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) =>
                          MailComposeScreen(initialRecipient: widget.userId),
                    ),
                  ),
            icon: const Icon(Icons.mail_outline),
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
                      displayUserName(user, fallback: widget.userId),
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                  ),
                  if (user.level != null)
                    Center(child: Text('等级 ${user.level}')),
                  const SizedBox(height: 16),
                  ListTile(
                    dense: true,
                    title: const Text('发帖数'),
                    trailing: Text('${user.postCount ?? 0}'),
                  ),
                  ListTile(
                    dense: true,
                    title: const Text('积分'),
                    trailing: Text('${user.score ?? 0}'),
                  ),
                  ListTile(
                    dense: true,
                    title: const Text('关注'),
                    trailing: Text('${user.followNum ?? 0}'),
                  ),
                  ListTile(
                    dense: true,
                    title: const Text('粉丝'),
                    trailing: Text('${user.fansNum ?? 0}'),
                  ),
                  ListTile(
                    dense: true,
                    title: const Text('最后登录'),
                    trailing: Text(
                      formatTimestamp(user.lastLoginTime, alwaysShowYear: true),
                    ),
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
