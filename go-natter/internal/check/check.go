package check

import (
	"context"
	"fmt"
	"io"

	"natter-openwrt/go-natter/internal/config"
)

func Run(context.Context, config.Config, io.Writer, io.Writer) error {
	return fmt.Errorf("Go natter check is not implemented yet")
}
