import 'dart:async';

import 'package:material_ui/material_ui.dart';
import 'package:byr_client/shared/storage/draft_store.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/data/models/attachment.dart';
import 'package:byr_client/shared/widgets/post_content.dart';

/// Post/reply/editor screen.
///
/// Mirrors the original editor: title, content, B/I/U formatting, link,
/// emoji panel, attachment upload, preview, signature/mail/outgo switches and
/// the anonymous option for boards that allow it.
class ComposeScreen extends StatefulWidget {
  const ComposeScreen({
    super.key,
    required this.boardName,
    this.reid,
    this.initialTitle,
    this.initialContent,
    this.editArticleId,
    this.allowAnonymous,
    this.initialAttachmentCount = 0,
    this.initialAttachments = const [],
  });

  final String boardName;

  /// Article id being replied to; null means a new thread.
  final String? reid;
  final String? initialTitle;
  final String? initialContent;

  /// When set, updates this article instead of creating a new one.
  final String? editArticleId;

  /// Whether the board allows anonymous posts. When null the composer fetches
  /// the board metadata itself.
  final bool? allowAnonymous;

  /// Number of attachments already on the edited article, used to number new
  /// `[upload=N]` markers.
  final int initialAttachmentCount;

  /// Existing attachments of the edited article, used by the preview.
  final List<Attachment> initialAttachments;

  @override
  State<ComposeScreen> createState() => _ComposeScreenState();
}

class _ComposeScreenState extends State<ComposeScreen> {
  late final TextEditingController _titleController = TextEditingController(
    text: widget.initialTitle,
  );
  late final TextEditingController _contentController = TextEditingController(
    text: widget.initialContent,
  );

  bool _busy = false;
  bool _uploading = false;
  String? _error;

  late final DraftStore _drafts;
  Timer? _draftTimer;
  bool _draftLoaded = false;
  bool _submitted = false;
  bool _anonymous = false;
  bool _allowAnonymous = false;
  bool _allowAttachment = true;
  bool _signature = false;
  bool _emailNotify = false;
  bool _outgo = false;
  String? _remainSpace;
  int? _remainCount;
  final List<FileElement> _uploadedFiles = [];
  final List<int> _uploadedIndexes = [];

  bool get _isReply => widget.reid != null;
  bool get _isEdit => widget.editArticleId != null;
  bool get _hasDraftContent =>
      _titleController.text.trim().isNotEmpty ||
      _contentController.text.trim().isNotEmpty ||
      _uploadedFiles.isNotEmpty;

  static final List<String> _emojiNames = _buildEmojiNames();

  static List<String> _buildEmojiNames() {
    final names = <String>[
      for (var i = 1; i <= 73; i++) 'em$i',
      for (var i = 0; i <= 41; i++) 'ema$i',
      for (var i = 0; i <= 24; i++) 'emb$i',
      'emb6r',
      for (var i = 1; i <= 58; i++) ...['emc$i', if (i == 50) 'emc50r'],
    ];
    return names;
  }

  String get _draftKey =>
      'composeDraft.${widget.boardName}.${widget.editArticleId ?? widget.reid ?? 'new'}';

  @override
  void initState() {
    super.initState();
    _drafts = AppScope.of(context).drafts;
    _titleController.addListener(_scheduleDraftSave);
    _contentController.addListener(_scheduleDraftSave);
    _restoreDraft();
    _allowAnonymous = widget.allowAnonymous ?? false;
    if (widget.allowAnonymous == null && !_isEdit) {
      _loadBoardRule();
    }
  }

  /// The board metadata carries `allow_anonymous`; the composer shows the
  /// checkbox only for boards that accept anonymous posts (e.g. IWhisper).
  Future<void> _loadBoardRule() async {
    final result = await AppScope.of(context).api.board
        .getBoard(board: widget.boardName);
    if (!mounted) return;
    setState(() {
      _allowAnonymous = result.data?.allowAnonymous ?? false;
      _allowAttachment = result.data?.allowAttachment ?? true;
    });
  }

  @override
  void dispose() {
    _draftTimer?.cancel();
    if (!_isEdit && _draftLoaded && !_submitted) _saveDraft();
    _titleController.dispose();
    _contentController.dispose();
    super.dispose();
  }

  void _scheduleDraftSave() {
    if (_isEdit || !_draftLoaded) return;
    _draftTimer?.cancel();
    _draftTimer = Timer(const Duration(milliseconds: 800), _saveDraft);
  }

  Future<void> _restoreDraft() async {
    if (_isEdit) {
      _draftLoaded = true;
      return;
    }
    final draft = await _drafts.load(_draftKey);
    if (!mounted) return;
    if (draft != null && !draft.isEmpty) {
      _titleController.text = draft.title;
      _contentController.text = draft.content;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _showMessage('???????????');
      });
    }
    _draftLoaded = true;
  }

  Future<void> _saveDraft() async {
    if (_isEdit) return;
    // Capture before awaiting: disposal must not read disposed controllers.
    final draft = ComposeDraft(
      title: _titleController.text,
      content: _contentController.text,
    );
    await _drafts.save(_draftKey, draft);
  }

  Future<void> _clearDraft() => _drafts.remove(_draftKey);

  void _showMessage(String text) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  void _insertText(String value) {
    final controller = _contentController;
    final selection = controller.selection;
    final text = controller.text;
    final start = selection.isValid ? selection.start : text.length;
    final end = selection.isValid ? selection.end : text.length;
    final next = text.replaceRange(start, end, value);
    controller.value = TextEditingValue(
      text: next,
      selection: TextSelection.collapsed(offset: start + value.length),
    );
  }

  void _wrapSelection(String prefix, String suffix) {
    final controller = _contentController;
    final selection = controller.selection;
    final text = controller.text;
    final start = selection.isValid ? selection.start : text.length;
    final end = selection.isValid ? selection.end : text.length;
    final selected = text.substring(start, end);
    final next = text.replaceRange(start, end, '$prefix$selected$suffix');
    controller.value = TextEditingValue(
      text: next,
      selection: TextSelection.collapsed(
        offset: start + prefix.length + selected.length,
      ),
    );
  }

  Future<void> _insertLink() async {
    final controller = TextEditingController();
    final url = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('插入链接'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(
            labelText: '网址',
            hintText: 'https://example.com',
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
            child: const Text('插入'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (url == null || url.isEmpty) return;
    final selection = _contentController.selection;
    final text = _contentController.text;
    final selected = selection.isValid
        ? text.substring(selection.start, selection.end)
        : '';
    _wrapSelection('[url=$url]', '[/url]');
    if (selected.isEmpty) {
      // `_wrapSelection` inserted an empty pair; put the URL as the label.
      final current = _contentController.text;
      _contentController.text = current.replaceFirst(
        '[url=$url][/url]',
        '[url=$url]$url[/url]',
      );
    }
  }

  Future<void> _pickEmoji() async {
    await showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('表情'),
        content: SizedBox(
          width: 460,
          height: 380,
          child: GridView.builder(
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 8,
              mainAxisSpacing: 4,
              crossAxisSpacing: 4,
            ),
            itemCount: _emojiNames.length,
            itemBuilder: (context, index) {
              final name = _emojiNames[index];
              return InkWell(
                onTap: () {
                  Navigator.of(context).pop();
                  _insertText('[em$name]');
                },
                child: Image.asset(
                  'assets/em/$name.gif',
                  fit: BoxFit.contain,
                  errorBuilder: (context, error, stack) => const SizedBox(),
                ),
              );
            },
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('关闭'),
          ),
        ],
      ),
    );
  }

  Future<void> _pickAttachment() async {
    if (_uploading || _busy) return;
    final picked = await AppScope.of(context).device.pickAttachments();
    if (!mounted) return;
    if (picked.isEmpty) return;
    for (final file in picked) {
      if (!mounted) return;
      await _upload(file.path, file.name);
    }
  }

  Future<void> _upload(String path, String name) async {
    setState(() {
      _uploading = true;
      _error = null;
    });
    final api = AppScope.of(context).api;
    final result = await api.forum.addAttachment(
      board: widget.boardName,
      // The official editor passes an empty id for a brand-new post.
      id: widget.editArticleId ?? widget.reid ?? '',
      filePath: path,
    );
    if (!mounted) return;
    setState(() => _uploading = false);
    if (result.isFailure) {
      _showMessage(result.message ?? '附件上传失败');
      return;
    }

    final files = result.data?.files ?? const <FileElement>[];
    final added = files.isEmpty
        ? [FileElement(name: name, url: null)]
        : files.toList();
    final start = widget.initialAttachmentCount + _uploadedFiles.length;
    setState(() {
      _uploadedFiles.addAll(added);
      for (var i = 0; i < added.length; i++) {
        _uploadedIndexes.add(start + i + 1);
      }
      _remainSpace = result.data?.remainSpace;
      _remainCount = result.data?.remainCount;
    });
    for (var i = 0; i < added.length; i++) {
      _insertText('[upload=${start + i + 1}][/upload]');
    }
    _showMessage('已上传：${added.map((f) => f.name ?? name).join(', ')}');
  }

  Future<void> _removeAttachment(FileElement file) async {
    final index = _uploadedFiles.indexOf(file);
    if (index < 0 || index >= _uploadedIndexes.length) return;
    final uploadIndex = _uploadedIndexes[index];
    final marker = '[upload=$uploadIndex][/upload]';

    // The attachment URL ends with the uploaded file id, e.g.
    // /open/attachment/Joke/742801/264.
    final fileId = _fileId(file.url);
    if (fileId != null) {
      final result = await AppScope.of(context).api.forum
          .deleteAttachment(board: widget.boardName, id: fileId);
      if (!mounted) return;
      if (result.isFailure) {
        _showMessage(result.message ?? '删除附件失败');
        return;
      }
    }

    setState(() {
      _uploadedFiles.removeAt(index);
      _uploadedIndexes.removeAt(index);
      _contentController.text = _contentController.text.replaceFirst(
        marker,
        '',
      );
    });
    _showMessage('已删除附件');
  }

  static String? _fileId(String? url) {
    if (url == null || url.isEmpty) return null;
    final path = Uri.tryParse(url)?.path ?? url;
    final segments = path.split('/').where((s) => s.isNotEmpty).toList();
    return segments.isEmpty ? null : segments.last;
  }

  void _preview() {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(
          _titleController.text.trim().isEmpty
              ? '预览'
              : _titleController.text.trim(),
        ),
        content: SizedBox(
          width: 640,
          child: SingleChildScrollView(
            child: PostContent(
              content: _contentController.text,
              attachments: [
                ...widget.initialAttachments,
                Attachment(files: _uploadedFiles),
              ],
              token: AppScope.of(context).api.client.token,
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('关闭'),
          ),
        ],
      ),
    );
  }

  Future<void> _submit() async {
    final content = _contentController.text.trim();
    final title = _titleController.text.trim();
    if (content.isEmpty || (!_isReply && title.isEmpty)) {
      setState(() => _error = _isReply ? '请输入回复内容' : '请输入标题和内容');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });

    final api = AppScope.of(context).api;
    final result = _isEdit
        ? await api.forum.updateArticle(
            board: widget.boardName,
            id: widget.editArticleId!,
            title: title,
            content: content,
          )
        : await api.forum.postArticle(
            board: widget.boardName,
            title: title,
            content: content,
            reid: widget.reid,
            anonymous: _anonymous,
            signature: _signature,
            email: _emailNotify,
            outgo: _outgo,
          );

    if (!mounted) return;
    if (result.isFailure) {
      setState(() {
        _busy = false;
        _error = result.message ?? '发送失败';
      });
      return;
    }

    await _clearDraft();
    _submitted = true;
    if (!mounted) return;
    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop) return;
        if (!_hasDraftContent) {
          if (mounted) Navigator.of(context).pop();
          return;
        }
        final leave = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: const Text('退出编辑'),
            content: const Text('尚未发送的内容已保存为草稿，确定退出？'),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(false),
                child: const Text('继续编辑'),
              ),
              FilledButton(
                onPressed: () => Navigator.of(context).pop(true),
                child: const Text('退出'),
              ),
            ],
          ),
        );
        if (leave == true && context.mounted) Navigator.of(context).pop();
      },
      child: Scaffold(
        appBar: AppBar(
          title: Text(_isEdit ? '编辑' : (_isReply ? '回复' : '发帖')),
          actions: [
            IconButton(
              onPressed: _preview,
              tooltip: '预览',
              icon: const Icon(Icons.visibility_outlined),
            ),
          ],
        ),
        body: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (!_isReply)
                TextField(
                  controller: _titleController,
                  textInputAction: TextInputAction.next,
                  decoration: const InputDecoration(
                    labelText: '标题',
                    border: OutlineInputBorder(),
                  ),
                ),
              if (!_isReply) const SizedBox(height: 12),
              if (_allowAnonymous && !_isEdit)
                CheckboxListTile(
                  value: _anonymous,
                  onChanged: _busy
                      ? null
                      : (value) => setState(() => _anonymous = value ?? false),
                  title: const Text('匿名发布'),
                  subtitle: const Text('以 IWhisper#编号 显示；不勾选则实名发布'),
                  controlAffinity: ListTileControlAffinity.leading,
                  contentPadding: EdgeInsets.zero,
                ),
              _buildToolbar(),
              const SizedBox(height: 4),
              Expanded(
                child: TextField(
                  controller: _contentController,
                  expands: true,
                  maxLines: null,
                  minLines: null,
                  textAlignVertical: TextAlignVertical.top,
                  decoration: const InputDecoration(
                    labelText: '内容',
                    alignLabelWithHint: true,
                    border: OutlineInputBorder(),
                  ),
                ),
              ),
              if (!_isEdit) ...[
                const SizedBox(height: 4),
                Wrap(
                  spacing: 8,
                  children: [
                    FilterChip(
                      label: const Text('附加签名'),
                      selected: _signature,
                      onSelected: _busy
                          ? null
                          : (value) => setState(() => _signature = value),
                    ),
                    FilterChip(
                      label: const Text('邮件通知'),
                      selected: _emailNotify,
                      onSelected: _busy
                          ? null
                          : (value) => setState(() => _emailNotify = value),
                    ),
                    FilterChip(
                      label: const Text('站外可见'),
                      selected: _outgo,
                      onSelected: _busy
                          ? null
                          : (value) => setState(() => _outgo = value),
                    ),
                  ],
                ),
              ],
              if (_uploadedFiles.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 4),
                  child: Wrap(
                    spacing: 6,
                    runSpacing: 4,
                    children: [
                      for (final file in _uploadedFiles)
                        InputChip(
                          avatar: const Icon(Icons.attach_file, size: 16),
                          label: Text(
                            file.name ?? '附件',
                            overflow: TextOverflow.ellipsis,
                          ),
                          onDeleted: _busy
                              ? null
                              : () => _removeAttachment(file),
                        ),
                    ],
                  ),
                ),
              if (_uploading)
                const Padding(
                  padding: EdgeInsets.only(top: 6),
                  child: LinearProgressIndicator(),
                ),
              if (_remainSpace != null || _remainCount != null)
                Padding(
                  padding: const EdgeInsets.only(top: 4),
                  child: Text(
                    '剩余空间 ${_remainSpace ?? '-'} · '
                    '剩余附件 ${_remainCount ?? '-'}',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ),
              const SizedBox(height: 12),
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Text(
                    _error!,
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.error,
                    ),
                  ),
                ),
              FilledButton(
                onPressed: _busy ? null : _submit,
                child: _busy
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(_isReply ? '发送回复' : '发布'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildToolbar() {
    return Wrap(
      spacing: 2,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        IconButton(
          tooltip: '粗体',
          onPressed: () => _wrapSelection('[b]', '[/b]'),
          icon: const Text('B', style: TextStyle(fontWeight: FontWeight.bold)),
        ),
        IconButton(
          tooltip: '斜体',
          onPressed: () => _wrapSelection('[i]', '[/i]'),
          icon: const Text('I', style: TextStyle(fontStyle: FontStyle.italic)),
        ),
        IconButton(
          tooltip: '下划线',
          onPressed: () => _wrapSelection('[u]', '[/u]'),
          icon: const Text(
            'U',
            style: TextStyle(decoration: TextDecoration.underline),
          ),
        ),
        IconButton(
          tooltip: '插入链接',
          onPressed: _insertLink,
          icon: const Icon(Icons.link),
        ),
        IconButton(
          tooltip: '表情',
          onPressed: _pickEmoji,
          icon: const Icon(Icons.emoji_emotions_outlined),
        ),
        if (_allowAttachment)
          IconButton(
            tooltip: '图片/附件',
            onPressed: _uploading ? null : _pickAttachment,
            icon: const Icon(Icons.attach_file),
          ),
        if (_uploading)
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: 8),
            child: SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          ),
      ],
    );
  }
}
