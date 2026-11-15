import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/search.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/user_avatar.dart';
import 'package:byr_client/features/thread/thread_screen.dart';
import 'package:byr_client/features/search/board_search_screen.dart';

class SearchScreen extends StatefulWidget {
  const SearchScreen({super.key});

  @override
  State<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends State<SearchScreen> {
  final _controller = TextEditingController();
  Future<Result<SearchArticle>>? _future;
  String _lastQuery = '';

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _search() {
    final query = _controller.text.trim();
    if (query.isEmpty) return;
    setState(() {
      _lastQuery = query;
      _future = AppScope.of(context).api.search.searchArticle(title1: query);
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: TextField(
          controller: _controller,
          autofocus: true,
          textInputAction: TextInputAction.search,
          onSubmitted: (_) => _search(),
          decoration: const InputDecoration(
            hintText: '搜索帖子标题',
            border: InputBorder.none,
          ),
        ),
        actions: [
          IconButton(
            tooltip: '搜索版面',
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute(builder: (_) => const BoardSearchScreen()),
            ),
            icon: const Icon(Icons.forum_outlined),
          ),
          IconButton(onPressed: _search, icon: const Icon(Icons.search)),
        ],
      ),
      body: _future == null
          ? const EmptyView(message: '输入关键词搜索帖子')
          : FutureBuilder<Result<SearchArticle>>(
              future: _future,
              builder: (context, snapshot) {
                if (snapshot.connectionState != ConnectionState.done) {
                  return const LoadingView();
                }
                return ResultView<SearchArticle>(
                  result: snapshot.data,
                  onRetry: _search,
                  builder: (data) {
                    if (data.articles.isEmpty) {
                      return EmptyView(message: '没有找到与“$_lastQuery”相关的帖子');
                    }
                    return ListView.separated(
                      itemCount: data.articles.length,
                      separatorBuilder: (_, _) => const Divider(height: 1),
                      itemBuilder: (context, index) {
                        final article = data.articles[index];
                        final board = article.boardName;
                        final id = article.id;
                        final time = formatTimestamp(article.postTime);
                        return ListTile(
                          leading: UserAvatar(
                            user: article.user,
                            radius: 16,
                            onTap: () => openUserProfile(context, article.user),
                          ),
                          title: Text(
                            article.title ?? '无标题',
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                          ),
                          subtitle: Text(
                            '${board ?? ''} · '
                            '${displayUserName(article.user, fallback: '')}'
                            '${time.isEmpty ? '' : ' · $time'}',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          onTap: board == null || id == null
                              ? null
                              : () {
                                  Navigator.of(context).push(
                                    MaterialPageRoute(
                                      builder: (_) => ThreadScreen(
                                        boardName: board,
                                        threadId: id,
                                      ),
                                    ),
                                  );
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
