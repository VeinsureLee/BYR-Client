import 'package:material_ui/material_ui.dart';

/// A text+icon reaction button with a small pop animation when the selected
/// state changes.
class ReactionButton extends StatelessWidget {
  const ReactionButton({
    super.key,
    required this.selected,
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.onPressed,
    this.busy = false,
    this.activeColor,
  });

  final bool selected;
  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final VoidCallback? onPressed;
  final bool busy;
  final Color? activeColor;

  @override
  Widget build(BuildContext context) {
    final color = selected
        ? (activeColor ?? Theme.of(context).colorScheme.primary)
        : null;
    return TextButton.icon(
      onPressed: busy ? null : onPressed,
      icon: AnimatedSwitcher(
        duration: const Duration(milliseconds: 220),
        switchInCurve: Curves.easeOutBack,
        switchOutCurve: Curves.easeIn,
        transitionBuilder: (child, animation) {
          return ScaleTransition(
            scale: animation,
            child: FadeTransition(opacity: animation, child: child),
          );
        },
        child: Icon(
          selected ? selectedIcon : icon,
          key: ValueKey<bool>(selected),
          size: 18,
          color: color,
        ),
      ),
      label: AnimatedDefaultTextStyle(
        duration: const Duration(milliseconds: 180),
        style: Theme.of(context).textTheme.labelLarge!.copyWith(color: color),
        child: Text(label),
      ),
    );
  }
}
