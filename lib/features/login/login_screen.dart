import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/app/app_home.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _usernameController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _usernameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  void _copy(String text) {
    AppScope.of(context).device.copyText(text);
    ScaffoldMessenger.of(context)
        .showSnackBar(const SnackBar(content: Text('已复制到剪贴板')));
  }

  Future<void> _login() async {
    final username = _usernameController.text.trim();
    final password = _passwordController.text;
    if (username.isEmpty || password.isEmpty) {
      setState(() => _error = '请输入用户名和密码');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });

    final api = AppScope.of(context).api;
    final result = await api.auth.login(username, password);
    if (!mounted) return;

    if (result.isFailure) {
      setState(() {
        _busy = false;
        _error = result.message ?? '登录失败';
      });
      return;
    }

    final token = result.data?.accessToken;
    if (token == null || token.isEmpty) {
      setState(() {
        _busy = false;
        _error = '服务器未返回登录令牌';
      });
      return;
    }

    api.client.token = token;
    await api.session.saveToken(token);
    await api.session.saveUsername(username);

    // Mirror the original post-login flow: fetch self info after writing
    // the token.
    try {
      await api.user.getSelfUserInfo();
    } catch (error) {
      // A failed profile fetch must not invalidate an otherwise good token.
      debugPrint('getSelfUserInfo failed: $error');
    }

    if (!mounted) return;
    Navigator.of(context)
        .pushReplacement(MaterialPageRoute(builder: (_) => const AppHome()));
  }

  Future<void> _diagnose() async {
    final username = _usernameController.text.trim();
    final password = _passwordController.text;
    if (username.isEmpty || password.isEmpty) {
      setState(() => _error = '请输入用户名和密码');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });
    final api = AppScope.of(context).api;
    final result = await api.auth.webLoginCheck(username, password);
    if (!mounted) return;
    setState(() => _busy = false);

    final title = result.ok ? '网页登录成功' : '网页登录失败';
    final detail = result.ok
        ? '账号密码正确。\n\n'
              '请关闭本窗口并点击「登录」——OAuth 登录通道已修复，'
              '现在应当可以正常获取访问令牌。'
        : '原因：${result.message}\n'
              '${result.code.isEmpty ? '' : 'ajax_code：${result.code}\n'}'
              '\n原始返回：\n${result.raw}';
    await showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: SingleChildScrollView(child: SelectableText(detail)),
        actions: [
          TextButton(
            onPressed: () => _copy('$title\n$detail'),
            child: const Text('复制'),
          ),
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('知道了'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final username = _usernameController.text.trim();
    final looksLikeEmail = username.contains('@');

    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'BYR',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.displaySmall?.copyWith(
                      color: theme.colorScheme.primary,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '北邮人论坛',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.titleMedium,
                  ),
                  const SizedBox(height: 32),
                  TextField(
                    controller: _usernameController,
                    textInputAction: TextInputAction.next,
                    autocorrect: false,
                    enableSuggestions: false,
                    textCapitalization: TextCapitalization.none,
                    onChanged: (_) => setState(() {}),
                    decoration: const InputDecoration(
                      labelText: '用户名',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  if (looksLikeEmail)
                    Padding(
                      padding: const EdgeInsets.only(top: 6),
                      child: Text(
                        '提示：请填写论坛用户名，而不是邮箱地址。',
                        style: TextStyle(color: theme.colorScheme.error),
                      ),
                    ),
                  const SizedBox(height: 16),
                  TextField(
                    controller: _passwordController,
                    obscureText: true,
                    autocorrect: false,
                    enableSuggestions: false,
                    textCapitalization: TextCapitalization.none,
                    keyboardType: TextInputType.visiblePassword,
                    onSubmitted: (_) => _busy ? null : _login(),
                    decoration: const InputDecoration(
                      labelText: '密码',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 20),
                  if (_error != null) ...[
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: theme.colorScheme.errorContainer.withValues(
                          alpha: 0.45,
                        ),
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          SelectableText(
                            _error!,
                            style: TextStyle(
                              color: theme.colorScheme.onErrorContainer,
                            ),
                          ),
                          Align(
                            alignment: Alignment.centerRight,
                            child: TextButton.icon(
                              onPressed: () => _copy(_error!),
                              icon: const Icon(Icons.copy, size: 16),
                              label: const Text('复制错误信息'),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 12),
                  ],
                  FilledButton(
                    onPressed: _busy ? null : _login,
                    child: _busy
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('登录'),
                  ),
                  TextButton(
                    onPressed: _busy ? null : _diagnose,
                    child: const Text('账号诊断（网页登录）'),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '登录失败时，错误信息可以直接选中复制，'
                    '也可以点击“复制错误信息”按钮。',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodySmall,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
