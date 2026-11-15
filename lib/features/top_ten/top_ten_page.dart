import 'package:material_ui/material_ui.dart';
import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/top_ten.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/user_avatar.dart';
import 'package:byr_client/features/thread/thread_screen.dart';

class TopTenPage extends StatefulWidget {
  const TopTenPage({super.key});

  @override
  State<TopTenPage> createState() => _TopTenPageState();
}

class _TopTenPageState extends State<TopTenPage> {
  Future<Result<TopTen>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.forum.getTopTen();
    setState(() {
      _future = future;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('十大')),
      body: FutureBuilder<Result<TopTen>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<TopTen>(
            result: snapshot.data,
            onRetry: _load,
            builder: (data) {
              if (data.articles.isEmpty) {
                return const EmptyView();
              }
              return ListView.separated(
                itemCount: data.articles.length,
                separatorBuilder: (_, _) => const Divider(height: 1),
                itemBuilder: (context, index) {
                  final article = data.articles[index];
                  final time = formatTimestamp(article.postTime);
                  return ListTile(
                    leading: UserAvatar(
                      user: article.user,
                      radius: 16,
                      onTap: () => openUserProfile(context, article.user),
                    ),
                    title: Text(article.title ?? '无标题'),
                    subtitle: Text(
                      '${article.boardName ?? ''} · '
                      '${displayUserName(article.user, fallback: '')}'
                      '${time.isEmpty ? '' : ' · $time'} · '
                      '${article.replyCount ?? 0} 回复',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () {
                      final board = article.boardName;
                      final id = article.id;
                      if (board != null && id != null) {
                        Navigator.of(context).push(
                          MaterialPageRoute(
                            builder: (_) =>
                                ThreadScreen(boardName: board, threadId: id),
                          ),
                        );
                      }
                    },
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
