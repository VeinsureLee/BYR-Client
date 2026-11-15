import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/storage/history_store.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/root_section.dart';
import 'package:byr_client/shared/data/models/section.dart';
import 'package:byr_client/shared/data/models/thread.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/post_content.dart';
import 'package:byr_client/shared/widgets/reaction_button.dart';
import 'package:byr_client/shared/widgets/user_avatar.dart';
import 'package:byr_client/features/bet/bet_screen.dart';
import 'package:byr_client/features/compose/compose_screen.dart';
import 'package:byr_client/features/vote/vote_screen.dart';

/// Three-column forum browser: sections -> boards -> threads -> detail.
class ForumWorkspace extends StatefulWidget {
  const ForumWorkspace({super.key});

  @override
  State<ForumWorkspace> createState() => _ForumWorkspaceState();
}

class _ForumWorkspaceState extends State<ForumWorkspace> {
  RootSection? _rootSection;
  String? _sectionName;
  Section? _section;
  String? _boardName;
  Board? _board;
  String? _threadId;
  Thread? _thread;

  bool _loading = true;
  String? _error;
  int _boardPage = 1;
  int _threadPage = 1;
  bool _collected = false;
  bool _collecting = false;

  @override
  void initState() {
    super.initState();
    _loadRoot();
  }

  Future<void> _loadRoot() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final result = await AppScope.of(context).api.section.getRootSection();
    if (!mounted) return;
    setState(() {
      _loading = false;
      if (result.isSuccess) {
        _rootSection = result.data;
      } else {
        _error = result.message;
      }
    });
  }

  Future<void> _selectSection(String name) async {
    setState(() {
      _sectionName = name;
      _section = null;
      _boardName = null;
      _board = null;
      _threadId = null;
      _thread = null;
      _error = null;
    });
    final result = await AppScope.of(context).api.section.getSection(name);
    if (!mounted || _sectionName != name) return;
    setState(() {
      if (result.isSuccess) {
        _section = result.data;
      } else {
        _error = result.message;
      }
    });
  }

  Future<void> _selectBoard(String name) async {
    setState(() {
      _boardName = name;
      _board = null;
      _threadId = null;
      _thread = null;
      _boardPage = 1;
      _collected = false;
      _error = null;
    });
    await _loadBoard();
  }

  Future<void> _loadBoard() async {
    final name = _boardName;
    if (name == null) return;
    final result = await AppScope.of(context).api.board
        .getBoard(board: name, page: _boardPage);
    if (!mounted || _boardName != name) return;
    setState(() {
      if (result.isSuccess) {
        _board = result.data;
      } else {
        _error = result.message;
      }
    });
  }

  Future<void> _selectThread(String id) async {
    setState(() {
      _threadId = id;
      _thread = null;
      _threadPage = 1;
      _collected = false;
      _error = null;
    });
    await _loadThread();
  }

  Future<void> _loadThread() async {
    final board = _boardName;
    final id = _threadId;
    if (board == null || id == null) return;
    final result = await AppScope.of(context).api.forum
        .getThread(board: board, id: id, page: _threadPage);
    if (!mounted || _threadId != id) return;
    setState(() {
      if (result.isSuccess) {
        _thread = result.data;
      } else {
        _error = result.message;
      }
    });
    final thread = result.data;
    if (result.isSuccess && thread != null) {
      AppScope.of(context).history.add(
        HistoryEntry(
          board: board,
          id: id,
          title: thread.title ?? '',
          visitedAt: DateTime.now().millisecondsSinceEpoch,
        ),
      );
    }
  }

  Future<void> _likeThread() async {
    final board = _boardName;
    final id = _threadId;
    if (board == null || id == null) return;
    final result = await AppScope.of(context).api.forum
        .likeArticle(board: board, id: id);
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          result.isSuccess
              ? '已赞（${result.data?.likeSum ?? 0}）'
              : (result.message ?? '点赞失败'),
        ),
      ),
    );
    if (result.isSuccess) await _loadThread();
  }

  Future<void> _collectThread() async {
    final board = _boardName;
    final id = _threadId;
    if (board == null || id == null || _collecting) return;
    final wasCollected = _collected;
    setState(() => _collecting = true);
    final api = AppScope.of(context).api;
    final result = wasCollected
        ? await api.forum.deleteCollection(board: board, id: id)
        : await api.forum.addCollection(board: board, id: id);
    if (!mounted) return;
    final message = result.message ?? '';
    setState(() {
      _collecting = false;
      if (result.isSuccess) {
        _collected = !wasCollected;
      } else if (message.contains('已经收录') || message.contains('已收藏')) {
        _collected = true;
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          result.isSuccess
              ? (wasCollected ? '已取消收藏' : '已加入收藏')
              : (message.contains('已经收录')
                    ? '已收藏（再次点击可取消）'
                    : (message.isEmpty ? '收藏失败' : message)),
        ),
      ),
    );
  }

  Future<void> _shareThread() async {
    final board = _boardName;
    final id = _threadId;
    if (board == null || id == null) return;
    await AppScope.of(context).device
        .copyText('https://bbs.byr.cn/article/$board/$id');
    if (!mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(const SnackBar(content: Text('帖子链接已复制')));
  }

  Future<void> _replyThread() async {
    final board = _boardName;
    final id = _threadId;
    if (board == null || id == null) return;
    final posted = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => ComposeScreen(boardName: board, reid: id),
      ),
    );
    if (posted == true) await _loadThread();
  }

  Future<void> _editThread() async {
    final board = _boardName;
    final id = _threadId;
    if (board == null || id == null) return;
    final result = await AppScope.of(context).api.forum
        .getArticle(board: board, id: id);
    if (!mounted) return;
    final article = result.data;
    if (result.isFailure || article == null) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(result.message ?? '无法加载帖子')));
      return;
    }
    final edited = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => ComposeScreen(
          boardName: board,
          editArticleId: id,
          initialTitle: article.title,
          initialContent: plainText(article.content),
          initialAttachmentCount: article.attachments.fold(
            0,
            (count, attachment) => count + attachment.files.length,
          ),
          initialAttachments: article.attachments,
        ),
      ),
    );
    if (edited == true) await _loadThread();
  }

  Future<void> _deleteThread() async {
    final board = _boardName;
    final id = _threadId;
    if (board == null || id == null) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('删除帖子'),
        content: const Text('确定删除本帖？此操作不可撤销。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    final result = await AppScope.of(context).api.forum
        .deleteArticle(board: board, id: id);
    if (!mounted) return;
    if (result.isFailure) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(result.message ?? '删除失败')));
      return;
    }
    setState(() {
      _threadId = null;
      _thread = null;
    });
    await _loadBoard();
  }

  void _openVote() {
    final id = _threadId;
    if (id == null) return;
    Navigator.of(context)
        .push(MaterialPageRoute(builder: (_) => VoteScreen(aid: id)));
  }

  void _openBet() {
    final id = _threadId;
    if (id == null) return;
    Navigator.of(context)
        .push(MaterialPageRoute(builder: (_) => BetScreen(bid: id)));
  }

  Widget _threadToolbar() {
    return Padding(
      padding: const EdgeInsets.fromLTRB(8, 4, 8, 0),
      child: Wrap(
        spacing: 4,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          TextButton.icon(
            onPressed: _likeThread,
            icon: const Icon(Icons.thumb_up_alt_outlined, size: 18),
            label: const Text('点赞'),
          ),
          ReactionButton(
            selected: _collected,
            icon: Icons.star_border,
            selectedIcon: Icons.star,
            label: _collected ? '已收藏' : '收藏',
            busy: _collecting,
            activeColor: Colors.amber,
            onPressed: _collectThread,
          ),
          TextButton.icon(
            onPressed: _replyThread,
            icon: const Icon(Icons.reply, size: 18),
            label: const Text('回复'),
          ),
          IconButton(
            onPressed: _shareThread,
            icon: const Icon(Icons.share_outlined),
            tooltip: '分享',
          ),
          IconButton(
            onPressed: _loadThread,
            icon: const Icon(Icons.refresh),
            tooltip: '刷新',
          ),
          PopupMenuButton<String>(
            tooltip: '更多',
            onSelected: (value) {
              switch (value) {
                case 'vote':
                  _openVote();
                  break;
                case 'bet':
                  _openBet();
                  break;
                case 'edit':
                  _editThread();
                  break;
                case 'delete':
                  _deleteThread();
                  break;
              }
            },
            itemBuilder: (_) => const [
              PopupMenuItem(value: 'vote', child: Text('投票')),
              PopupMenuItem(value: 'bet', child: Text('竞猜')),
              PopupMenuItem(value: 'edit', child: Text('编辑本帖')),
              PopupMenuItem(value: 'delete', child: Text('删除本帖')),
            ],
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const LoadingView();
    if (_rootSection == null) {
      return ErrorView(message: _error ?? '加载分区失败', onRetry: _loadRoot);
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SizedBox(
          width: 220,
          child: _Column(
            title: '分区',
            child: ListView(
              children: [
                for (final section in _rootSection!.sections)
                  ListTile(
                    dense: true,
                    selected: section.name == _sectionName,
                    title: Text(
                      section.description?.isNotEmpty == true
                          ? section.description!
                          : (section.name ?? '未命名'),
                    ),
                    onTap: section.name == null
                        ? null
                        : () => _selectSection(section.name!),
                  ),
              ],
            ),
          ),
        ),
        const VerticalDivider(width: 1),
        SizedBox(
          width: 240,
          child: _Column(
            title: _section?.description?.isNotEmpty == true
                ? _section!.description!
                : (_sectionName ?? '版面'),
            child: _section == null
                ? const EmptyView(message: '选择分区')
                : ListView(
                    children: [
                      for (final board in _section!.boards) _boardTile(board),
                      for (final sub in _section!.subSections) ...[
                        Padding(
                          padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
                          child: Text(
                            sub.description?.isNotEmpty == true
                                ? sub.description!
                                : (sub.name ?? '子分区'),
                            style: Theme.of(context).textTheme.labelMedium,
                          ),
                        ),
                        for (final board in sub.boards) _boardTile(board),
                      ],
                    ],
                  ),
          ),
        ),
        const VerticalDivider(width: 1),
        SizedBox(
          width: 340,
          child: _Column(
            title: _board?.description?.isNotEmpty == true
                ? _board!.description!
                : (_boardName ?? '主题'),
            child: _board == null
                ? const EmptyView(message: '选择版面')
                : Column(
                    children: [
                      Expanded(
                        child: ListView.separated(
                          itemCount: _board!.articles.length,
                          separatorBuilder: (_, _) => const Divider(height: 1),
                          itemBuilder: (context, index) {
                            final article = _board!.articles[index];
                            final time = formatTimestamp(article.postTime);
                            return ListTile(
                              dense: true,
                              selected: article.id == _threadId,
                              leading: UserAvatar(
                                user: article.user,
                                radius: 14,
                                onTap: () =>
                                    openUserProfile(context, article.user),
                              ),
                              title: Text(
                                article.title ?? '无标题',
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                              ),
                              subtitle: Text(
                                '${displayUserName(article.user, fallback: '')}'
                                '${time.isEmpty ? '' : ' · $time'}'
                                ' · ${article.replyCount ?? 0} 回复',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                              onTap: article.id == null
                                  ? null
                                  : () => _selectThread(article.id!),
                            );
                          },
                        ),
                      ),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          TextButton(
                            onPressed: _boardPage > 1
                                ? () {
                                    setState(() => _boardPage--);
                                    _loadBoard();
                                  }
                                : null,
                            child: const Text('上一页'),
                          ),
                          Text('$_boardPage'),
                          TextButton(
                            onPressed: () {
                              setState(() => _boardPage++);
                              _loadBoard();
                            },
                            child: const Text('下一页'),
                          ),
                        ],
                      ),
                    ],
                  ),
          ),
        ),
        const VerticalDivider(width: 1),
        Expanded(
          child: _Column(
            title: _thread?.title ?? '帖子',
            child: _thread == null
                ? const EmptyView(message: '选择主题')
                : Column(
                    children: [
                      _threadToolbar(),
                      Expanded(
                        child: ListView(
                          padding: const EdgeInsets.all(16),
                          children: [
                            for (final post in _thread!.articles)
                              _PostTile(post: post, onChanged: _loadThread),
                          ],
                        ),
                      ),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          TextButton(
                            onPressed: _threadPage > 1
                                ? () {
                                    setState(() => _threadPage--);
                                    _loadThread();
                                  }
                                : null,
                            child: const Text('上一页'),
                          ),
                          Text('$_threadPage'),
                          TextButton(
                            onPressed: () {
                              setState(() => _threadPage++);
                              _loadThread();
                            },
                            child: const Text('下一页'),
                          ),
                        ],
                      ),
                    ],
                  ),
          ),
        ),
      ],
    );
  }

  Widget _boardTile(Board board) {
    final name = board.name;
    if (name == null) return const SizedBox.shrink();
    return ListTile(
      dense: true,
      selected: name == _boardName,
      title: Text(name),
      subtitle: board.description == null
          ? null
          : Text(
              board.description!,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
      onTap: () => _selectBoard(name),
    );
  }
}

class _Column extends StatelessWidget {
  const _Column({required this.title, required this.child});

  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Container(
          width: double.infinity,
          padding: const EdgeInsets.all(12),
          color: Theme.of(context).colorScheme.surfaceContainerHighest,
          child: Text(
            title,
            style: Theme.of(context).textTheme.titleSmall
                ?.copyWith(fontWeight: FontWeight.bold),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        Expanded(child: child),
      ],
    );
  }
}

class _PostTile extends StatefulWidget {
  const _PostTile({required this.post, this.onChanged});

  final Article post;
  final VoidCallback? onChanged;

  @override
  State<_PostTile> createState() => _PostTileState();
}

class _PostTileState extends State<_PostTile> {
  late int _likeSum = widget.post.likeSum ?? 0;
  late bool _liked = widget.post.isLiked ?? false;
  bool _liking = false;

  late int _voteDownSum = widget.post.votedownSum ?? 0;
  late bool _votedDown = widget.post.isVotedown ?? false;
  bool _votingDown = false;

  void _showMessage(String text) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _like() async {
    final post = widget.post;
    final board = post.boardName;
    final id = post.id;
    if (board == null || id == null || _liking) return;
    if (_liked) {
      _showMessage('已赞过，服务器不支持取消赞');
      return;
    }
    setState(() => _liking = true);
    final result = await AppScope.of(context).api.forum
        .likeArticle(board: board, id: id);
    if (!mounted) return;
    setState(() {
      _liking = false;
      if (result.isSuccess) {
        _liked = true;
        _votedDown = false;
        _likeSum = result.data?.likeSum ?? _likeSum + 1;
        _voteDownSum = result.data?.votedownSum ?? _voteDownSum;
      } else if ((result.message ?? '').contains('已赞过')) {
        _liked = true;
      }
    });
    _showMessage(
      result.isSuccess ? '已赞（$_likeSum）' : (result.message ?? '点赞失败'),
    );
  }

  Future<void> _voteDown() async {
    final post = widget.post;
    final board = post.boardName;
    final id = post.id;
    if (board == null || id == null || _votingDown) return;
    if (_votedDown) {
      _showMessage('已踩过，服务器不支持取消踩');
      return;
    }
    setState(() => _votingDown = true);
    final result = await AppScope.of(context).api.forum
        .voteDownArticle(board: board, id: id);
    if (!mounted) return;
    setState(() {
      _votingDown = false;
      if (result.isSuccess) {
        _votedDown = true;
        _liked = false;
        _voteDownSum = result.data?.votedownSum ?? _voteDownSum + 1;
        _likeSum = result.data?.likeSum ?? _likeSum;
      } else if ((result.message ?? '').contains('已踩过')) {
        _votedDown = true;
      }
    });
    _showMessage(
      result.isSuccess ? '已踩（$_voteDownSum）' : (result.message ?? '操作失败'),
    );
  }

  Future<void> _reply() async {
    final post = widget.post;
    final board = post.boardName;
    final id = post.id;
    if (board == null || id == null) return;
    final posted = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => ComposeScreen(boardName: board, reid: id),
      ),
    );
    if (posted == true) widget.onChanged?.call();
  }

  @override
  Widget build(BuildContext context) {
    final post = widget.post;
    final postTime = formatTimestamp(post.postTime);
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                UserAvatar(
                  user: post.user,
                  radius: 18,
                  onTap: () => openUserProfile(context, post.user),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      GestureDetector(
                        onTap: () => openUserProfile(context, post.user),
                        child: Text(
                          displayUserName(post.user),
                          style: Theme.of(context).textTheme.titleSmall
                              ?.copyWith(fontWeight: FontWeight.bold),
                        ),
                      ),
                      if (postTime.isNotEmpty)
                        Text(
                          postTime,
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                    ],
                  ),
                ),
                if (post.position != null) Text('#${post.position}'),
              ],
            ),
            const SizedBox(height: 8),
            PostContent(
              content: post.content,
              attachments: post.attachments,
              token: AppScope.of(context).api.client.token,
            ),
            const SizedBox(height: 4),
            Wrap(
              spacing: 4,
              children: [
                ReactionButton(
                  selected: _liked,
                  icon: Icons.thumb_up_alt_outlined,
                  selectedIcon: Icons.thumb_up,
                  label: '赞 $_likeSum',
                  busy: _liking,
                  onPressed: _like,
                ),
                ReactionButton(
                  selected: _votedDown,
                  icon: Icons.thumb_down_alt_outlined,
                  selectedIcon: Icons.thumb_down,
                  label: '踩 $_voteDownSum',
                  busy: _votingDown,
                  activeColor: Theme.of(context).colorScheme.error,
                  onPressed: _voteDown,
                ),
                TextButton.icon(
                  onPressed: _reply,
                  icon: const Icon(Icons.reply, size: 16),
                  label: const Text('回复'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
