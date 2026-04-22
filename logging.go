package configx

func logDebug(opts *Options, msg string, attrs ...any) {
	if opts == nil || opts.logger == nil || !opts.debug {
		return
	}
	opts.logger.Debug(msg, attrs...)
}

func logError(opts *Options, msg string, attrs ...any) {
	if opts == nil || opts.logger == nil {
		return
	}
	opts.logger.Error(msg, attrs...)
}
