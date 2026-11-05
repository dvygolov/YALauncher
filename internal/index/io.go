package index

import "os"

// readSmallFile reads at most limit bytes: shortcuts and .url files are tiny,
// so nothing here has to stream.

func osReadFileLimit(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > 0 && st.Size() < limit {
		limit = st.Size()
	}
	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if n == 0 && err != nil {
		return nil, err
	}
	return buf[:n], nil
}
