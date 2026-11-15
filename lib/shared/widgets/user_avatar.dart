import 'package:cached_network_image/cached_network_image.dart';
import 'package:material_ui/material_ui.dart';

import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/features/user/user_profile_screen.dart';
import 'package:byr_client/shared/widgets/common.dart';

/// Opens the profile page for a real user; no-op for anonymous whispers.
void openUserProfile(BuildContext context, User? user) {
  final id = user?.id;
  if (id == null || id.isEmpty || isAnonymousUser(user)) return;
  Navigator.of(context)
      .push(MaterialPageRoute(builder: (_) => UserProfileScreen(userId: id)));
}

/// Circular user avatar with a fallback icon.
///
/// Anonymous 悄悄话 users (`IWhisper#123`) have no `user_name` or face, and
/// must not navigate to a profile page.
class UserAvatar extends StatelessWidget {
  const UserAvatar({
    super.key,
    required this.user,
    this.radius = 18,
    this.onTap,
  });

  final User? user;
  final double radius;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final anonymous = isAnonymousUser(user);
    final face = anonymous ? null : absoluteUrl(user?.faceUrl);
    final size = radius * 2;
    final fallback = Container(
      width: size,
      height: size,
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      alignment: Alignment.center,
      child: Icon(
        anonymous ? Icons.visibility_off_outlined : Icons.person_outline,
        size: radius * 1.1,
        color: Theme.of(context).colorScheme.onSurfaceVariant,
      ),
    );

    final avatar = ClipOval(
      child: SizedBox(
        width: size,
        height: size,
        child: face == null || face.isEmpty
            ? fallback
            : CachedNetworkImage(
                imageUrl: face,
                httpHeaders: const {'Referer': 'https://bbs.byr.cn/'},
                fit: BoxFit.cover,
                placeholder: (context, _) => fallback,
                errorWidget: (context, _, _) => fallback,
              ),
      ),
    );

    final canTap = !anonymous && (user?.id ?? '').isNotEmpty && onTap != null;
    return InkWell(
      onTap: canTap ? onTap : null,
      customBorder: const CircleBorder(),
      child: avatar,
    );
  }
}
