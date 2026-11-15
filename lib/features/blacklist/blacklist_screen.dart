import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/black_list.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/user_avatar.dart';

class BlackListScreen extends StatefulWidget {
  const BlackListScreen({super.key});

  @override
  State<BlackListScreen> createState() => _BlackListScreenState();
}

class _BlackListScreenState extends State<BlackListScreen> {
  Future<Result<BlackList>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.blackList.getBlackList();
    setState(() {
      _future = future;
    });
  }

  Future<void> _remove(String id) async {
    final result = await AppScope.of(context).api.blackList.deleteBlackList(id);
    if (!mounted) return;
    if (result.isFailure) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(result.message ?? '操作失败')));
      return;
    }
    _load();
  }

  Future<void> _add() async {
    final controller = TextEditingController();
    final id = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('加入黑名单'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(
            labelText: '用户名',
            hintText: '论坛 ID',
            border: OutlineInputBorder(),
          ),
          onSubmitted: (value) => Navigator.of(context).pop(value.trim()),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(controller.text.trim()),
            child: const Text('添加'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (id == null || id.isEmpty || !mounted) return;
    final result = await AppScope.of(context).api.blackList.addBlackList(id);
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(result.isSuccess ? '已加入黑名单' : (result.message ?? '操作失败')),
      ),
    );
    if (result.isSuccess) _load();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('黑名单'),
        actions: [
          IconButton(
            tooltip: '添加',
            onPressed: _add,
            icon: const Icon(Icons.person_add_alt_1_outlined),
          ),
        ],
      ),
      body: FutureBuilder<Result<BlackList>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<BlackList>(
            result: snapshot.data,
            onRetry: _load,
            builder: (data) {
              if (data.users.isEmpty) {
                return const EmptyView(message: '黑名单为空');
              }
              return ListView.separated(
                itemCount: data.users.length,
                separatorBuilder: (_, _) => const Divider(height: 1),
                itemBuilder: (context, index) {
                  final user = data.users[index];
                  return ListTile(
                    leading: UserAvatar(
                      user: user,
                      radius: 18,
                      onTap: () => openUserProfile(context, user),
                    ),
                    title: Text(displayUserName(user)),
                    trailing: IconButton(
                      icon: const Icon(Icons.delete_outline),
                      onPressed: user.id == null
                          ? null
                          : () => _remove(user.id!),
                    ),
                  );
                },
              );
            },
          );
        },
      ),
    );
  }
}
