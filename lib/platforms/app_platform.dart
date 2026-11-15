/// Supported native hosts. Layout and plugin configuration are composed once.
enum AppPlatform {
  android,
  windows,
  macos,
  linux;

  bool get isDesktop => this != android;
}
